import { createAction } from "../act";
import { createEffect, createSignal, For, on, Show } from "solid-js";
import { Browser } from "@wailsio/runtime";
import { EnrichService } from "../../bindings/camstuart/talent-hound";
import type { EnrichOutcome, EnrichPreview } from "../../bindings/camstuart/talent-hound";
import type { Identity } from "../../bindings/camstuart/talent-hound/internal/models";
import { bumpWorkspace } from "../workspaceRevision";

// The public handles a candidate is known by, and the one action that reads
// them. Enrichment is the second thing in this application that sends a
// person's information out — a handle — so it has the same shape as a search:
// a preview of exactly what would go and where, then a confirmation.
const PROVIDERS = [
  { value: "", label: "Detect from the URL" },
  { value: "github", label: "GitHub" },
  { value: "website", label: "Website" },
  { value: "linkedin", label: "LinkedIn" },
  { value: "hn", label: "Hacker News" },
];

export default function IdentitiesSection(props: { candidateId: number }) {
  const [identities, setIdentities] = createSignal<Identity[]>([]);
  const [provider, setProvider] = createSignal("");
  const [url, setUrl] = createSignal("");
  const [preview, setPreview] = createSignal<EnrichPreview | null>(null);
  const [outcome, setOutcome] = createSignal<EnrichOutcome | null>(null);
  // The backend's own words, verbatim: it knows rules the UI does not.
  const { act, reloader, error, busy } = createAction();

  const reload = reloader(async (isCurrent) => {
    const [list, p] = await Promise.all([
      EnrichService.Identities(props.candidateId),
      EnrichService.Preview(props.candidateId),
    ]);
    if (!isCurrent()) return;
    setIdentities((list ?? []) as Identity[]);
    setPreview((p ?? null) as EnrichPreview | null);
  });

  createEffect(
    on(
      () => props.candidateId,
      () => {
        setOutcome(null);
        void reload();
      },
    ),
  );

  const add = (e: Event) => {
    e.preventDefault();
    return act(async () => {
      await EnrichService.AddIdentity(props.candidateId, provider(), url());
      setUrl("");
      await reload();
    });
  };

  const remove = (id: number) =>
    act(async () => {
      await EnrichService.RemoveIdentity(id);
      await reload();
    });

  const open = (target: string) => act(() => Browser.OpenURL(target));

  const [confirming, setConfirming] = createSignal(false);

  const run = () =>
    act(async () => {
      try {
        const result = (await EnrichService.Run(props.candidateId)) as EnrichOutcome;
        setOutcome(result);
        setConfirming(false);
        bumpWorkspace();
      } finally {
        await reload();
      }
    });

  const outcomeText = (o: EnrichOutcome) => {
    const added = o.artifactIds?.length ?? 0;
    const parts = [
      added === 0 ? "nothing new" : `${added} ${added === 1 ? "artifact" : "artifacts"} added`,
      o.unchanged ? `${o.unchanged} unchanged` : "",
      o.partial ? `stopped early: ${o.failureReason}` : "",
    ].filter(Boolean);
    return parts.join(", ");
  };

  return (
    <section class="record-section" aria-label="Identities">
      <h3>Identities</h3>
      <p class="muted">
        Public handles this person is known by. They are how a search result is recognised as someone already
        here, and what enrichment reads.
      </p>

      <Show when={error()}>
        <p class="modal-error" role="alert">{error()}</p>
      </Show>

      <ul class="record-list" aria-label="Identity list">
        <For each={identities()} fallback={<li class="muted">No identities recorded.</li>}>
          {(id) => (
            <li class="search-hit" aria-label={`Identity ${id.provider} ${id.handle}`}>
              <span class="artifact-name">
                {id.provider}: {id.handle}
                <Show when={id.verifiedAt}>
                  <span class="muted"> — confirmed {id.verifiedAt}</span>
                </Show>
              </span>
              <div class="search-bar">
                <button aria-label={`Open ${id.provider} ${id.handle}`} onClick={() => open(id.url)}>
                  Open
                </button>
                <button aria-label={`Remove ${id.provider} ${id.handle}`} disabled={busy()} onClick={() => remove(id.id)}>
                  Remove
                </button>
              </div>
            </li>
          )}
        </For>
      </ul>

      <form class="search-bar" aria-label="Add identity" onSubmit={add}>
        <select aria-label="Identity provider" value={provider()} onChange={(e) => setProvider(e.currentTarget.value)}>
          <For each={PROVIDERS}>{(p) => <option value={p.value}>{p.label}</option>}</For>
        </select>
        <input
          aria-label="Identity URL"
          placeholder="https://github.com/login"
          value={url()}
          onInput={(e) => setUrl(e.currentTarget.value)}
        />
        <button type="submit" aria-label="Add this identity" disabled={busy() || !url().trim()}>
          Add
        </button>
      </form>

      <div class="search-bar">
        <button
          aria-label="Enrich from GitHub"
          disabled={busy() || !!preview()?.reason}
          title={preview()?.reason ?? ""}
          onClick={() => setConfirming(true)}
        >
          Enrich from GitHub
        </button>
        <Show when={preview()?.reason}>
          <span class="muted" aria-label="Enrich unavailable reason">{preview()?.reason}</span>
        </Show>
      </div>

      <Show when={confirming() && preview()}>
        {(p) => (
          <div class="extraction-view" role="region" aria-label="Enrichment preview">
            <h4>This is exactly what will be sent</h4>
            <p class="muted">
              The {(p().handles ?? []).length === 1 ? "handle" : "handles"}{" "}
              <span class="artifact-name">{(p().handles ?? []).join(", ")}</span> to GitHub, asking for:
            </p>
            <ul class="record-list" aria-label="Enrichment endpoints">
              <For each={p().endpoints}>{(e) => <li class="muted">{e}</li>}</For>
            </ul>
            <p class="muted">
              What comes back is kept as evidence under this candidate — profile, repositories, activity — and never
              written into their fields. The audit record says a handle was sent, not which.
            </p>
            <button class="primary" aria-label="Run enrichment" disabled={busy()} onClick={run}>
              Send it
            </button>
            <button aria-label="Cancel enrichment" onClick={() => setConfirming(false)}>
              Cancel
            </button>
          </div>
        )}
      </Show>

      <Show when={outcome()}>
        {(o) => (
          <p class="muted" aria-label="Enrichment outcome">
            {outcomeText(o())}
          </p>
        )}
      </Show>
    </section>
  );
}
