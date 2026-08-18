import { createEffect, createSignal, For, Show } from "solid-js";
import { CriteriaService, RecordService } from "../../bindings/camstuart/talent-hound";
import type { Proposal } from "../../bindings/camstuart/talent-hound";
import type { Candidate, SearchCriterion } from "../../bindings/camstuart/talent-hound/internal/models";
import { workspaceRevision } from "../workspaceRevision";

// What the recruiter is looking for, kept apart from what any document says
// about anyone. A resume saying someone worked at Northwind is not a statement
// that they want to again.
//
// Two kinds of red on this screen and they mean different things. A refusal is
// final and has no way past it. A warning is advice from a model that may be
// wrong, attached to a criterion that is stored and in use.
export default function CriteriaPanel(props: { initiativeId: number }) {
  const [criteria, setCriteria] = createSignal<SearchCriterion[]>([]);
  const [version, setVersion] = createSignal(0);
  const [candidates, setCandidates] = createSignal<Candidate[]>([]);
  const [candidate, setCandidate] = createSignal(0);
  const [proposals, setProposals] = createSignal<Proposal[]>([]);
  const [chosen, setChosen] = createSignal<Record<number, boolean>>({});
  const [text, setText] = createSignal("");
  const [priority, setPriority] = createSignal("must_have");
  const [editing, setEditing] = createSignal<number | null>(null);
  const [draft, setDraft] = createSignal("");
  const [refusal, setRefusal] = createSignal("");
  const [error, setError] = createSignal("");

  // Every action reports the backend's own words: it knows rules the UI does not.
  const act = async (run: () => Promise<unknown>) => {
    setError("");
    setRefusal("");
    try {
      await run();
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      // A refusal is shown as a refusal, not as a generic failure: it is a rule
      // rather than something that went wrong.
      if (message.includes("cannot be a search criterion")) setRefusal(message);
      else setError(message);
    }
  };

  const reload = () =>
    act(async () => {
      setCriteria(((await CriteriaService.List(props.initiativeId)) ?? []) as SearchCriterion[]);
      setVersion(await CriteriaService.Version(props.initiativeId));
      const list = ((await RecordService.ListCandidates()) ?? []) as Candidate[];
      setCandidates(list);
      if (!candidate() && list.length > 0) setCandidate(list[0].id);
    });

  createEffect(() => {
    workspaceRevision();
    void reload();
  });

  const add = () =>
    act(async () => {
      const wording = text().trim();
      if (!wording) return;
      await CriteriaService.Add({
        initiativeId: props.initiativeId,
        text: wording,
        priority: priority(),
      } as never);
      setText("");
      await reload();
    });

  const saveEdit = (id: number, currentPriority: string) =>
    act(async () => {
      await CriteriaService.Edit(id, draft(), currentPriority);
      setEditing(null);
      await reload();
    });

  const setPriorityOf = (id: number, next: string) =>
    act(async () => {
      const row = criteria().find((c) => c.id === id);
      if (!row) return;
      await CriteriaService.Edit(id, row.text, next);
      await reload();
    });

  const remove = (id: number) =>
    act(async () => {
      await CriteriaService.Remove(id);
      await reload();
    });

  // Reordering is presentation only — it deliberately does not move the version.
  const move = (id: number, by: number) =>
    act(async () => {
      const ids = criteria().map((c) => c.id);
      const at = ids.indexOf(id);
      const to = at + by;
      if (at < 0 || to < 0 || to >= ids.length) return;
      [ids[at], ids[to]] = [ids[to], ids[at]];
      await CriteriaService.Reorder(props.initiativeId, ids);
      await reload();
    });

  const propose = () =>
    act(async () => {
      setProposals(((await CriteriaService.Propose(props.initiativeId, candidate())) ?? []) as Proposal[]);
      setChosen({});
    });

  // Nothing here applies on its own: the recruiter names what to take.
  const applyChosen = () =>
    act(async () => {
      const take = proposals().filter((_, i) => chosen()[i]);
      if (take.length === 0) return;
      await CriteriaService.Apply(props.initiativeId, take as never);
      setProposals([]);
      setChosen({});
      await reload();
    });

  return (
    <section class="record-section" aria-label="Search criteria">
      <h3>Search criteria</h3>
      <p class="muted" aria-label="Criteria version">
        What this initiative is looking for — version {version()}. Criteria are the recruiter's intent and are kept
        separate from what any profile says.
      </p>

      <div class="search-bar">
        <input
          aria-label="New criterion"
          placeholder="Something the role needs"
          value={text()}
          onInput={(e) => setText(e.currentTarget.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") add();
          }}
        />
        <select aria-label="Priority to add with" value={priority()} onChange={(e) => setPriority(e.currentTarget.value)}>
          <option value="must_have">Must have</option>
          <option value="nice_to_have">Nice to have</option>
        </select>
        <button class="primary" aria-label="Add this criterion" onClick={add}>
          Add
        </button>
      </div>

      <Show when={refusal()}>
        <p class="modal-error" aria-label="Refused criterion">
          {refusal()} There is no way to add this one.
        </p>
      </Show>
      <Show when={error()}>
        <p class="modal-error">{error()}</p>
      </Show>

      <ul class="record-list" aria-label="Criteria">
        <For each={criteria()} fallback={<li class="muted">No criteria yet.</li>}>
          {(c, i) => (
            <li class="search-hit">
              <span class="artifact-name">
                <span class="muted">{c.priority === "must_have" ? "Must have" : "Nice to have"} — </span>
              </span>
              <Show
                when={editing() === c.id}
                fallback={
                  <>
                    {/* Recruiter-written text: displayed, never rendered. */}
                    <pre aria-label={`Criterion ${i() + 1}`}>{c.text}</pre>
                    <button
                      aria-label={`Edit criterion ${i() + 1}`}
                      onClick={() => {
                        setEditing(c.id);
                        setDraft(c.text);
                      }}
                    >
                      Edit
                    </button>
                    <button
                      aria-label={`Make criterion ${i() + 1} ${c.priority === "must_have" ? "nice to have" : "must have"}`}
                      onClick={() => setPriorityOf(c.id, c.priority === "must_have" ? "nice_to_have" : "must_have")}
                    >
                      {c.priority === "must_have" ? "Make nice to have" : "Make must have"}
                    </button>
                    <button aria-label={`Move criterion ${i() + 1} up`} onClick={() => move(c.id, -1)}>
                      Up
                    </button>
                    <button aria-label={`Move criterion ${i() + 1} down`} onClick={() => move(c.id, 1)}>
                      Down
                    </button>
                    <button aria-label={`Remove criterion ${i() + 1}`} onClick={() => remove(c.id)}>
                      Remove
                    </button>
                  </>
                }
              >
                <input
                  aria-label={`Wording for criterion ${i() + 1}`}
                  value={draft()}
                  onInput={(e) => setDraft(e.currentTarget.value)}
                  onKeyDown={(e) => {
                    if (e.key === "Enter") saveEdit(c.id, c.priority);
                    if (e.key === "Escape") setEditing(null);
                  }}
                />
                <button class="primary" aria-label={`Save criterion ${i() + 1}`} onClick={() => saveEdit(c.id, c.priority)}>
                  Save
                </button>
                <button aria-label={`Cancel editing criterion ${i() + 1}`} onClick={() => setEditing(null)}>
                  Cancel
                </button>
              </Show>
              <Show when={c.warning}>
                {/* Advice, not a rule: this criterion is stored and in use. */}
                <p class="shell-note" aria-label={`Warning about criterion ${i() + 1}`}>
                  Possible proxy — {c.warning}
                </p>
              </Show>
            </li>
          )}
        </For>
      </ul>

      <div class="search-bar">
        <select aria-label="Propose from candidate" value={String(candidate())} onFocus={() => void reload()} onChange={(e) => setCandidate(Number(e.currentTarget.value))}>
          <For each={candidates()} fallback={<option value="0">No candidates yet</option>}>
            {(c) => <option value={String(c.id)}>{c.fullName}</option>}
          </For>
        </select>
        <button aria-label="Propose criteria from this candidate's approved profile" onClick={propose}>
          Propose from profile
        </button>
      </div>

      <Show when={proposals().length > 0}>
        <div class="extraction-view" role="region" aria-label="Proposed criteria">
          <h4>
            Proposed criteria
            <button aria-label="Discard these proposals" onClick={() => setProposals([])}>
              Discard
            </button>
          </h4>
          <p class="muted">Nothing here is a criterion until you apply it.</p>
          <ul class="record-list">
            <For each={proposals()}>
              {(p, i) => (
                <li>
                  <label>
                    <input
                      type="checkbox"
                      aria-label={`Apply proposal ${i() + 1}`}
                      checked={!!chosen()[i()]}
                      onChange={(e) => setChosen({ ...chosen(), [i()]: e.currentTarget.checked })}
                    />{" "}
                    <span class="muted">from {p.from} — </span>
                    {p.text}
                  </label>
                </li>
              )}
            </For>
          </ul>
          <button class="primary" aria-label="Apply the chosen proposals" onClick={applyChosen}>
            Apply chosen
          </button>
        </div>
      </Show>
    </section>
  );
}
