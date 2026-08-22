import { createAction } from "../act";
import { createEffect, createSignal, For, Show } from "solid-js";
import { DeletionService, RecordService } from "../../bindings/camstuart/talent-hound";
import type { Preview, SharedArtifactChoice } from "../../bindings/camstuart/talent-hound";
import type { Candidate, Role } from "../../bindings/camstuart/talent-hound/internal/models";
import { bumpWorkspace, workspaceRevision } from "../workspaceRevision";

// The one operation with no undo, so the preview is the moment to change your
// mind — and it lists exactly what would go rather than saying "and related
// data".
type Target = { kind: "candidate" | "role"; id: number; label: string };

export default function DeletePanel() {
  const [candidates, setCandidates] = createSignal<Candidate[]>([]);
  const [roles, setRoles] = createSignal<Role[]>([]);
  const [target, setTarget] = createSignal<Target | null>(null);
  const [preview, setPreview] = createSignal<Preview | null>(null);
  const [done, setDone] = createSignal("");
  // The backend's own words, verbatim: it knows rules the UI does not.
  const { act, reloader, error, busy } = createAction();

  const reload = reloader(async (isCurrent) => {
    const candidates = ((await RecordService.ListCandidates()) ?? []) as Candidate[];
    if (!isCurrent()) return;
    setCandidates(candidates);
    const roles = ((await RecordService.ListRoles()) ?? []) as Role[];
    if (!isCurrent()) return;
    setRoles(roles);
  });

  createEffect(() => {
    workspaceRevision();
    void reload();
  });

  const previewFor = (t: Target) =>
    act(async () => {
      setDone("");
      setTarget(t);
      const p =
        t.kind === "candidate"
          ? await DeletionService.PreviewCandidate(t.id)
          : await DeletionService.PreviewRolePurge(t.id);
      setPreview((p ?? null) as Preview | null);
    });

  const confirm = (choice: SharedArtifactChoice) =>
    act(async () => {
      const t = target();
      if (!t) return;
      if (t.kind === "candidate") {
        await DeletionService.DeleteCandidate(t.id, choice);
      } else {
        await DeletionService.PurgeRole(t.id);
      }
      setDone(`${t.label} was deleted.`);
      setPreview(null);
      setTarget(null);
      bumpWorkspace();
      await reload();
    });

  return (
    <section class="record-section" aria-label="Delete">
      <h3>Delete</h3>
      <p class="muted">
        Nothing here can be undone. Every deletion is previewed first, and the preview lists exactly what would go.
      </p>

      <Show when={error()}>
        <p class="modal-error" role="alert">{error()}</p>
      </Show>
      <Show when={done()}>
        <p class="muted" aria-label="Deletion outcome">
          {done()}
        </p>
      </Show>

      <ul class="record-list" aria-label="Deletable records">
        <For each={candidates()} fallback={<li class="muted">No candidates.</li>}>
          {(c) => (
            <li class="search-hit">
              <span class="artifact-name">{c.fullName}</span>
              <button
                aria-label={`Preview deleting ${c.fullName}`}
                disabled={busy()}
                onClick={() => previewFor({ kind: "candidate", id: c.id, label: c.fullName })}
              >
                Delete this candidate…
              </button>
            </li>
          )}
        </For>
        <For each={roles()}>
          {(r) => (
            <li class="search-hit">
              <span class="artifact-name">{r.title}</span>
              <button
                aria-label={`Preview purging ${r.title}`}
                disabled={busy()}
                onClick={() => previewFor({ kind: "role", id: r.id, label: r.title })}
              >
                Purge this role…
              </button>
            </li>
          )}
        </For>
      </ul>

      <Show when={preview()}>
        {(p) => (
          <div class="extraction-view" role="region" aria-label="Deletion preview">
            <h4>
              What deleting {target()?.label} would do
              <button aria-label="Cancel this deletion" onClick={() => setPreview(null)}>
                Cancel
              </button>
            </h4>

            <Show when={(p().blockers ?? []).length > 0}>
              {/* A refusal that says "cannot delete" and stops is one the
                  recruiter cannot act on. */}
              <ul class="record-list" aria-label="What is blocking this deletion">
                <For each={p().blockers ?? []}>{(b) => <li class="modal-error">{b}</li>}</For>
              </ul>
            </Show>

            <ul class="record-list" aria-label="What would be removed">
              <For each={p().removes ?? []}>
                {(c) => (
                  <li class="muted">
                    {c.kind}: {c.count}
                    <Show when={(c.detail ?? []).length > 0}>
                      <span> — {(c.detail ?? []).join(", ")}</span>
                    </Show>
                  </li>
                )}
              </For>
            </ul>

            <Show
              when={p().choice}
              fallback={
                <Show when={!p().blocked}>
                  <button class="primary" aria-label="Confirm this deletion" onClick={() => confirm("" as SharedArtifactChoice)}>
                    Delete permanently
                  </button>
                </Show>
              }
            >
              {/* Neither default is safe, so the application refuses to choose. */}
              <p class="shell-note" aria-label="Choice required">
                {p().choice}
              </p>
              <button
                aria-label="Delete the shared artifacts everywhere"
                onClick={() => confirm("delete_everywhere" as SharedArtifactChoice)}
              >
                Delete them everywhere
              </button>
              <button
                aria-label="Keep the shared artifacts under their other links"
                onClick={() => confirm("retain_under_other_links" as SharedArtifactChoice)}
              >
                Keep them under their other links
              </button>
            </Show>
          </div>
        )}
      </Show>
    </section>
  );
}
