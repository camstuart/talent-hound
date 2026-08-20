import { createAction } from "../act";
import { createEffect, createSignal, For, Show } from "solid-js";
import { ShortlistService, RecordService } from "../../bindings/camstuart/talent-hound";
import type { Shortlist } from "../../bindings/camstuart/talent-hound";
import type { Candidate } from "../../bindings/camstuart/talent-hound/internal/models";
import { workspaceRevision } from "../workspaceRevision";

// Twenty roles worth the expensive stage, and — more importantly — why each one
// is here. A shortlist a recruiter cannot interrogate is a shortlist they
// cannot defend.
export default function ShortlistPanel(props: { initiativeId: number }) {
  const [candidates, setCandidates] = createSignal<Candidate[]>([]);
  const [candidate, setCandidate] = createSignal(0);
  const [shortlist, setShortlist] = createSignal<Shortlist | null>(null);
  // The backend's own words, verbatim: it knows rules the UI does not.
  const { act, error, busy } = createAction();

  const reload = () =>
    act(async () => {
      setCandidates(((await RecordService.ListCandidates()) ?? []) as Candidate[]);
    });

  createEffect(() => {
    workspaceRevision();
    void reload();
  });

  const build = () =>
    act(async () => {
      setShortlist(((await ShortlistService.Build(props.initiativeId, candidate())) ?? null) as Shortlist | null);
    });

  return (
    <section class="record-section" aria-label="Shortlist">
      <h3>Shortlist</h3>
      <p class="muted">
        The roles worth assessing, chosen from the ones in scope by your criteria. Roles that conflict with a
        must-have stay on the list so you can see and reject them.
      </p>

      <div class="search-bar">
        <select
          aria-label="Shortlist for candidate"
          value={String(candidate())}
          onFocus={() => void reload()}
          onChange={(e) => setCandidate(Number(e.currentTarget.value))}
        >
          <option value="0">Criteria only</option>
          <For each={candidates()}>{(c) => <option value={String(c.id)}>{c.fullName}</option>}</For>
        </select>
        <button class="primary" aria-label="Build the shortlist" disabled={busy()} onClick={build}>
          Build the shortlist
        </button>
      </div>

      <Show when={error()}>
        <p class="modal-error" role="alert">{error()}</p>
      </Show>

      <Show when={shortlist()}>
        {(s) => (
          <>
            <p class="muted" aria-label="Shortlist scope">
              {(s().entries ?? []).length} of {s().eligible} roles in scope, under criteria version{" "}
              {s().criteriaVersion}.
            </p>
            <ul class="record-list" aria-label="Shortlisted roles">
              <For
                each={s().entries ?? []}
                fallback={<li class="muted">Nothing matched. The roles in scope are still there — try wider criteria.</li>}
              >
                {(entry) => (
                  <li class="search-hit">
                    <span class="artifact-name">
                      {entry.position}. {entry.title}
                    </span>
                    <Show when={(entry.conflicts ?? []).length > 0}>
                      {/* Shown, not filtered: a role you would reject is more
                          useful than an empty list. */}
                      <p class="shell-note" aria-label={`Conflicts for ${entry.title}`}>
                        <For each={entry.conflicts ?? []}>
                          {(c) => (
                            <span>
                              {c.field}: this role says {c.found}, you asked for {c.wanted}.{" "}
                            </span>
                          )}
                        </For>
                      </p>
                    </Show>
                    <ul aria-label={`Why ${entry.title} is here`}>
                      <For each={entry.why ?? []}>
                        {(w) => (
                          <li class="muted">
                            {w.method} match at rank {w.rank} for “{w.source}”
                          </li>
                        )}
                      </For>
                    </ul>
                  </li>
                )}
              </For>
            </ul>
          </>
        )}
      </Show>
    </section>
  );
}
