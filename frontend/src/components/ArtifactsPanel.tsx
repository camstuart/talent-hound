import { createEffect, createSignal, For, onCleanup, onMount, Show } from "solid-js";
import { workspaceRevision } from "../workspaceRevision";
import { latestOnly } from "../latestOnly";
import { ArtifactService, ExtractService } from "../../bindings/camstuart/talent-hound";
import { ExtractionState, LinkTarget } from "../../bindings/camstuart/talent-hound/internal/models";
import type { Artifact } from "../../bindings/camstuart/talent-hound/internal/models";

// How often the panel asks whether an extraction it started has finished. It
// polls only while one is outstanding, and stops the moment none is.
const POLL_MS = 400;

// readBase64 hands the file's bytes to the backend unchanged. A data URL is the
// shortest path to base64 that copes with binary content.
const readBase64 = (file: File) =>
  new Promise<string>((resolve, reject) => {
    const reader = new FileReader();
    reader.onerror = () => reject(reader.error ?? new Error("the file could not be read"));
    reader.onload = () => resolve(String(reader.result).split(",")[1] ?? "");
    reader.readAsDataURL(file);
  });

// base64OfText encodes UTF-8 text the way the backend expects to receive it.
const base64OfText = (text: string) => {
  const bytes = new TextEncoder().encode(text);
  let binary = "";
  bytes.forEach((b) => (binary += String.fromCharCode(b)));
  return btoa(binary);
};

const sizeLabel = (bytes: number) =>
  bytes < 1024 ? `${bytes} B` : bytes < 1024 * 1024 ? `${Math.round(bytes / 1024)} KB` : `${(bytes / 1048576).toFixed(1)} MB`;

// Artifacts attached to one initiative — or, when a CRM record is given
// instead, to that record. Attaching and detaching move links only: the bytes
// are stored once and never copied.
export default function ArtifactsPanel(props: { initiativeId?: number; target?: { type: LinkTarget; id: number } }) {
  const target = () => props.target ?? { type: LinkTarget.LinkInitiative, id: props.initiativeId! };
  const [artifacts, setArtifacts] = createSignal<Artifact[]>([]);
  const [orphans, setOrphans] = createSignal<Artifact[]>([]);
  const [renamingId, setRenamingId] = createSignal<number | null>(null);
  const [pasted, setPasted] = createSignal("");
  const [pastedName, setPastedName] = createSignal("");
  const [error, setError] = createSignal("");
  // Artifacts whose extraction the recruiter has just asked for. Every other
  // artifact is pending too — pending is simply "nobody has read this yet" —
  // so this, not the state, is what says whether anything is in flight.
  const [extracting, setExtracting] = createSignal<number[]>([]);
  const [viewing, setViewing] = createSignal<{ name: string; markdown: string } | null>(null);

  // Three reasons to reload — the mount, the workspace revision, and the
  // extraction poll — so the same rules the records list needs apply here, and
  // this list never had either of them.
  const reload = latestOnly(async (isCurrent) => {
    const attached = (await ArtifactService.ListForTarget(target().type, target().id)) ?? [];
    const unattached = (await ArtifactService.ListOrphans()) ?? [];
    if (!isCurrent()) return;
    setArtifacts(attached);
    setOrphans(unattached);
    const settled = new Set(
      [...attached, ...unattached].filter((a) => a.extractionState !== ExtractionState.ExtractionPending).map((a) => a.id),
    );
    setExtracting((ids) => ids.filter((id) => !settled.has(id)));
  });
  // Another panel may attach an artifact — a dropped resume, for one — so this
  // list follows the workspace revision as well as its own actions. It also
  // follows its own target: a CRM caller keeps this component mounted across
  // a change of selected record, rather than remounting it, so the target
  // itself has to be a reload trigger too.
  createEffect(() => {
    workspaceRevision();
    target();
    void reload();
  });

  onMount(() => {
    void reload();
    const timer = setInterval(() => {
      if (extracting().length > 0) void reload();
    }, POLL_MS);
    onCleanup(() => clearInterval(timer));
  });

  // Every action reports the backend's own words: it knows rules the UI does not.
  const act = async (run: () => Promise<unknown>) => {
    setError("");
    try {
      await run();
      await reload();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    }
  };

  const upload = (e: Event & { currentTarget: HTMLInputElement }) => {
    const file = e.currentTarget.files?.[0];
    e.currentTarget.value = "";
    if (!file) return;
    return act(async () =>
      ArtifactService.Create({
        displayName: "",
        originalFilename: file.name,
        source: "Uploaded by the recruiter",
        dataBase64: await readBase64(file),
        targetType: target().type,
        targetId: target().id,
      }),
    );
  };

  const ingestPasted = () =>
    act(async () => {
      const created = await ArtifactService.Create({
        displayName: pastedName(),
        originalFilename: "",
        source: "Pasted by the recruiter",
        dataBase64: base64OfText(pasted()),
        targetType: target().type,
        targetId: target().id,
      });
      setPasted("");
      setPastedName("");
      return created;
    });

  const extract = (a: Artifact) =>
    act(async () => {
      const job = await ExtractService.Extract(a.id, props.initiativeId ?? 0);
      setExtracting((ids) => [...ids, a.id]);
      return job;
    });

  // The extraction is text a stranger wrote: it is shown in a <pre> and never
  // rendered as markup, because nothing here interprets it.
  const view = (a: Artifact) =>
    act(async () => setViewing({ name: a.displayName, markdown: await ExtractService.Markdown(a.id) }));

  const extractionLabel = (a: Artifact) => {
    if (extracting().includes(a.id)) return "extracting";
    if (a.extractionState === ExtractionState.ExtractionFailed) return `not read (${a.extractionError})`;
    if (a.extractionState === ExtractionState.ExtractionExtracted) return "read";
    return "not read";
  };

  const artifactRow = (a: Artifact, attached: boolean) => (
    <li class="artifact-row">
      <Show
        when={renamingId() === a.id}
        fallback={
          <span class="artifact-name">
            {a.displayName}
            <span class="muted">
              {" "}
              — {a.mediaType.split(";")[0]}, {sizeLabel(a.byteLength)}, {extractionLabel(a)}
            </span>
          </span>
        }
      >
        <input
          class="rename-input"
          aria-label={`New name for ${a.displayName}`}
          value={a.displayName}
          ref={(el) => setTimeout(() => el.select())}
          onKeyDown={(e) => {
            if (e.key === "Escape") setRenamingId(null);
            if (e.key === "Enter") {
              const value = e.currentTarget.value;
              setRenamingId(null);
              act(() => ArtifactService.Rename(a.id, value));
            }
          }}
        />
      </Show>
      <button aria-label={`Rename ${a.displayName}`} onClick={() => setRenamingId(a.id)}>
        Rename
      </button>
      <Show when={a.extractionState !== ExtractionState.ExtractionExtracted && !extracting().includes(a.id)}>
        <button aria-label={`Extract ${a.displayName}`} onClick={() => extract(a)}>
          {a.extractionState === ExtractionState.ExtractionFailed ? "Extract again" : "Extract"}
        </button>
      </Show>
      <Show when={a.extractionState === ExtractionState.ExtractionExtracted}>
        <button aria-label={`View extraction of ${a.displayName}`} onClick={() => view(a)}>
          View text
        </button>
      </Show>
      <Show when={attached}>
        <button
          aria-label={`Detach ${a.displayName}`}
          title="Removes this link only — the bytes and any other links are kept"
          onClick={() => act(() => ArtifactService.Detach(a.id, target().type, target().id))}
        >
          Detach
        </button>
      </Show>
      <Show when={!attached}>
        <button
          aria-label={`Attach ${a.displayName}`}
          onClick={() => act(() => ArtifactService.Link(a.id, target().type, target().id))}
        >
          Attach here
        </button>
      </Show>
    </li>
  );

  return (
    <div class="artifacts">
      <section class="record-section" aria-label="Artifacts">
        <h3>Artifacts</h3>
        <ul class="record-list">
          <For each={artifacts()} fallback={<li class="muted">Nothing attached to this initiative yet.</li>}>
            {(a) => artifactRow(a, true)}
          </For>
        </ul>

        <label class="record-field">
          <span>Attach a file</span>
          <input type="file" aria-label="Attach a file" onChange={upload} />
        </label>

        <div class="record-form">
          <h4>Paste text</h4>
          <label class="record-field">
            <span>Name</span>
            <input
              aria-label="Pasted text name"
              value={pastedName()}
              onInput={(e) => setPastedName(e.currentTarget.value)}
            />
          </label>
          <label class="record-field">
            <span>Text</span>
            <textarea aria-label="Pasted text" value={pasted()} onInput={(e) => setPasted(e.currentTarget.value)} />
          </label>
          <div class="record-form-actions">
            <button class="primary" disabled={!pastedName().trim()} onClick={ingestPasted}>
              Add pasted text
            </button>
          </div>
        </div>

        <Show when={error()}>
          <p class="modal-error" role="alert">{error()}</p>
        </Show>

        <Show when={viewing()}>
          {(v) => (
            <div class="extraction-view">
              <h4>
                {v().name} — extracted text
                <button aria-label="Close the extraction view" onClick={() => setViewing(null)}>
                  Close
                </button>
              </h4>
              {/* Untrusted: displayed, never rendered, never acted on. */}
              <pre aria-label="Extracted text">{v().markdown}</pre>
            </div>
          )}
        </Show>
      </section>

      <section class="record-section" aria-label="Orphaned artifacts">
        <h3>Orphaned artifacts</h3>
        <p class="muted">Artifacts with no links. Their bytes are kept until they are explicitly deleted.</p>
        <ul class="record-list">
          <For each={orphans()} fallback={<li class="muted">No orphaned artifacts.</li>}>
            {(a) => artifactRow(a, false)}
          </For>
        </ul>
      </section>
    </div>
  );
}
