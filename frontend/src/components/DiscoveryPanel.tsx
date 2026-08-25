import { createAction } from "../act";
import { createEffect, createSignal, For, Show } from "solid-js";
import QueryPreviewEditor from "./QueryPreviewEditor";
import PastSearches from "./PastSearches";
import { DiscoveryService, RecordService } from "../../bindings/camstuart/talent-hound";
import type { QueryPreview, SearchOutcome } from "../../bindings/camstuart/talent-hound";
import type { Candidate, Search } from "../../bindings/camstuart/talent-hound/internal/models";
import { bumpWorkspace, workspaceRevision } from "../workspaceRevision";

// The first screen where something leaves the machine, and the design follows
// from that: the recruiter sees the exact query, edits it if they want, and
// nothing is sent until they confirm.
//
// The two warnings are deliberately different. Naming a company is a legitimate
// search that discloses where you are looking. Naming a person discloses who
// you are looking for, and that is the thing this whole path exists to prevent
// happening by accident.
export default function DiscoveryPanel(props: { initiativeId: number }) {
  const [candidates, setCandidates] = createSignal<Candidate[]>([]);
  const [candidate, setCandidate] = createSignal(0);
  const [preview, setPreview] = createSignal<QueryPreview | null>(null);
  const [query, setQuery] = createSignal("");
  const [outcome, setOutcome] = createSignal<SearchOutcome | null>(null);
  const [searches, setSearches] = createSignal<Search[]>([]);
  // The backend's own words, verbatim: it knows rules the UI does not.
  const { act, reloader, error, busy, setError } = createAction();

  const reload = reloader(async (isCurrent) => {
    const [list, searches] = await Promise.all([
      RecordService.ListCandidates(),
      DiscoveryService.Searches(props.initiativeId),
    ]);
    if (!isCurrent()) return;
    setCandidates((list ?? []) as Candidate[]);
    setSearches((searches ?? []) as Search[]);
  });

  createEffect(() => {
    workspaceRevision();
    void reload();
  });

  const build = () =>
    act(async () => {
      setOutcome(null);
      const p = (await DiscoveryService.Preview(props.initiativeId, candidate())) as QueryPreview;
      setPreview(p);
      setQuery(p.query);
    });

  // Editing re-inspects: the warnings are about what is in the box now, not
  // about what was generated.
  const edit = (text: string) =>
    act(async () => {
      setQuery(text);
      setPreview(((await DiscoveryService.Inspect(candidate(), text)) ?? null) as QueryPreview | null);
    });

  // Cancelling is the absence of the operation: it calls nothing at all, so
  // there is no request and no record of one.
  const cancel = () => {
    setPreview(null);
    setQuery("");
    setError("");
  };

  const send = () =>
    act(async () => {
      try {
        const result = (await DiscoveryService.Send({
          initiativeId: props.initiativeId,
          candidateId: candidate(),
          query: query(),
          limit: 20,
        } as never)) as SearchOutcome;
        setOutcome(result);
        setPreview(null);
        bumpWorkspace();
      } finally {
        // A failed request was still transmitted, and the attempt is recorded
        // with its reason — so the list is refreshed either way, or a failure
        // would look like a search that never happened.
        await reload();
      }
    });

  return (
    <section class="record-section" aria-label="Role discovery">
      <h3>Find roles</h3>
      <p class="muted">
        Queries are built from approved evidence and criteria, with direct identifiers removed and organizations
        generalized. Nothing is sent until you have seen the exact text and confirmed it.
      </p>

      <div class="search-bar">
        <select
          aria-label="Search for candidate"
          value={String(candidate())}
          onFocus={() => void reload()}
          onChange={(e) => setCandidate(Number(e.currentTarget.value))}
        >
          <option value="0">Criteria only</option>
          <For each={candidates()}>{(c) => <option value={String(c.id)}>{c.fullName}</option>}</For>
        </select>
        <button aria-label="Build a query" disabled={busy()} onClick={build}>
          Build a query
        </button>
      </div>

      <Show when={error()}>
        <p class="modal-error" role="alert">{error()}</p>
      </Show>

      <Show when={preview()}>
        {(p) => (
          <QueryPreviewEditor
            regionLabel="Query preview"
            fieldLabel="Query to send"
            sendLabel="Send this query"
            cancelLabel="Cancel this search"
            preview={p()}
            query={query()}
            busy={busy()}
            onEdit={edit}
            onSend={send}
            onCancel={cancel}
          />
        )}
      </Show>

      <Show when={outcome()}>
        {(o) => (
          <p class="muted" aria-label="Search outcome">
            {o().created} new {o().created === 1 ? "role" : "roles"}, {o().updated} updated
            <Show when={o().partial}>
              {" "}
              — {o().skipped} {o().skipped === 1 ? "record" : "records"} could not be read, so this result is
              incomplete
            </Show>
          </p>
        )}
      </Show>

      <PastSearches
        label="Past searches"
        emptyText="No searches yet."
        queryLabel={(i) => `Query sent for search ${i}`}
        searches={searches()}
      />
    </section>
  );
}
