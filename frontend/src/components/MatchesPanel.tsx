import { createAction } from "../act";
import { createEffect, createSignal, For, Show } from "solid-js";
import { AssessService, RecordService } from "../../bindings/camstuart/talent-hound";
import type { Candidate, Match, MatchResult } from "../../bindings/camstuart/talent-hound/internal/models";
import { workspaceRevision } from "../workspaceRevision";

// The conclusions the recruiter acts on, in both directions, each carrying what
// it cites.
//
// A result cannot be checked by looking at it — only by looking at its
// evidence — so the evidence is one click away from every claim, and a result
// whose inputs have changed says so rather than looking current.
const DIRECTION_LABELS: Record<string, string> = {
  role_fits_candidate: "Does this role suit them",
  candidate_fits_role: "Do they suit this role",
};

const RESULT_LABELS: Record<string, string> = {
  met: "met",
  not_met: "not met",
  unknown: "no evidence either way",
};

export default function MatchesPanel(props: { initiativeId: number }) {
  const [candidates, setCandidates] = createSignal<Candidate[]>([]);
  const [candidate, setCandidate] = createSignal(0);
  const [matches, setMatches] = createSignal<Match[]>([]);
  const [shown, setShown] = createSignal<number | null>(null);
  const [evidenceOf, setEvidenceOf] = createSignal<string | null>(null);
  // The backend's own words, verbatim: it knows rules the UI does not.
  const { act, reloader, error, busy } = createAction();

  const reload = reloader(async (isCurrent) => {
    // Sequential on purpose: which candidate is chosen can be settled by
    // setting the list, so the second call cannot be issued alongside the first.
    const list = ((await RecordService.ListCandidates()) ?? []) as Candidate[];
    if (!isCurrent()) return;
    setCandidates(list);
    const chosen = candidate();
    if (!chosen) return;
    const matches = ((await AssessService.Matches(props.initiativeId, chosen)) ?? []) as Match[];
    if (!isCurrent()) return;
    setMatches(matches);
  });

  createEffect(() => {
    workspaceRevision();
    void reload();
  });

  const assessAll = () =>
    act(async () => {
      await AssessService.AssessAll(props.initiativeId, candidate());
      // The job runs in the background; the list catches up when it lands.
      setTimeout(() => void reload(), 800);
    });

  const resultsFor = (match: Match, direction: string) =>
    (match.results ?? []).filter((r: MatchResult) => r.direction === direction);

  const citationsOf = (result: MatchResult): { ref: string; text: string }[] => {
    try {
      return JSON.parse(result.citations ?? "[]");
    } catch {
      return [];
    }
  };

  return (
    <section class="record-section" aria-label="Matches">
      <h3>Matches</h3>
      <p class="muted">
        Both directions, per requirement, with the evidence each result rests on. A match whose inputs have changed
        says so rather than looking current.
      </p>

      <div class="search-bar">
        <select
          aria-label="Matches for candidate"
          value={String(candidate())}
          onFocus={() => void reload()}
          onChange={(e) => {
            setCandidate(Number(e.currentTarget.value));
            void reload();
          }}
        >
          <option value="0">Choose a candidate</option>
          <For each={candidates()}>{(c) => <option value={String(c.id)}>{c.fullName}</option>}</For>
        </select>
        <button class="primary" aria-label="Assess the shortlist" disabled={busy() || !candidate()} onClick={assessAll}>
          Assess the shortlist
        </button>
      </div>

      <Show when={error()}>
        <p class="modal-error" role="alert">{error()}</p>
      </Show>

      <ul class="record-list" aria-label="Assessed matches">
        <For each={matches()} fallback={<li class="muted">No assessed matches yet.</li>}>
          {(match, i) => (
            <li class="search-hit">
              <span class="artifact-name">
                {i() + 1}. {match.roleTitle || `Role ${match.roleId}`}
                <span class="muted">
                  {" "}
                  — {match.unmetMustHaves} unmet must-{match.unmetMustHaves === 1 ? "have" : "haves"},{" "}
                  {match.unknownMustHaves} unknown, {match.metNiceToHaves} nice-to-haves met
                </span>
              </span>
              <Show when={match.stale}>
                <p class="shell-note" aria-label={`Staleness of match ${i() + 1}`}>
                  Something this assessment depended on has changed — assess again to bring it up to date.
                </p>
              </Show>

              <button
                aria-label={`Show the assessment of match ${i() + 1}`}
                onClick={() => setShown(shown() === i() ? null : i())}
              >
                {shown() === i() ? "Hide" : "Show"} the assessment
              </button>

              <Show when={shown() === i()}>
                <For each={["role_fits_candidate", "candidate_fits_role"]}>
                  {(direction) => (
                    <div class="extraction-view">
                      <h4>{DIRECTION_LABELS[direction]}</h4>
                      <ul class="record-list" aria-label={`${DIRECTION_LABELS[direction]} for match ${i() + 1}`}>
                        <For
                          each={resultsFor(match, direction)}
                          fallback={<li class="muted">Nothing assessed in this direction.</li>}
                        >
                          {(r, j) => (
                            <li>
                              <span class="artifact-name">
                                {RESULT_LABELS[r.result] ?? r.result}
                                <span class="muted"> — {r.priority.replace(/_/g, " ")}</span>
                              </span>
                              {/* Untrusted: displayed, never rendered. */}
                              <pre aria-label={`Requirement ${j() + 1} of ${DIRECTION_LABELS[direction]}`}>
                                {r.requirement}
                              </pre>
                              <Show when={r.reason}>
                                <p class="muted" data-provenance="ai">{r.reason}</p>
                              </Show>
                              <Show when={citationsOf(r).length > 0}>
                                <button
                                  aria-label={`Show the evidence for requirement ${j() + 1} of ${DIRECTION_LABELS[direction]}`}
                                  onClick={() =>
                                    setEvidenceOf(evidenceOf() === `${i()}:${direction}:${j()}` ? null : `${i()}:${direction}:${j()}`)
                                  }
                                >
                                  Evidence
                                </button>
                              </Show>
                              <Show when={evidenceOf() === `${i()}:${direction}:${j()}`}>
                                <For each={citationsOf(r)}>
                                  {(c) => (
                                    <pre data-provenance="source" aria-label={`Cited evidence ${c.ref}`}>{c.text}</pre>
                                  )}
                                </For>
                              </Show>
                            </li>
                          )}
                        </For>
                      </ul>
                    </div>
                  )}
                </For>
              </Show>
            </li>
          )}
        </For>
      </ul>
    </section>
  );
}
