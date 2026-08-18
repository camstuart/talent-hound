import { createAction } from "../act";
import { createSignal, For, onMount, Show } from "solid-js";
import { CandidateProfileService, RecordService } from "../../bindings/camstuart/talent-hound";
import { bumpWorkspace } from "../workspaceRevision";
import type { AspectCitation, Diff, Readiness } from "../../bindings/camstuart/talent-hound";
import type { Candidate, Profile } from "../../bindings/camstuart/talent-hound/internal/models";

// Where a person decides whether the model's account of a candidate is true
// enough to act on. Everything shown is text out of a document a stranger
// wrote, or text the recruiter typed: displayed, never rendered.
//
// The state labels are deliberately plain. "Proposed" has to read as "nobody
// has checked this yet" without a tooltip, because the whole gate depends on a
// recruiter knowing which of these they are looking at.
export default function CandidateProfilePanel(props: { initiativeId: number }) {
  const [candidates, setCandidates] = createSignal<Candidate[]>([]);
  const [selected, setSelected] = createSignal(0);
  const [profileVersion, setProfileVersion] = createSignal<Profile | null>(null);
  const [readiness, setReadiness] = createSignal<Readiness | null>(null);
  const [citations, setCitations] = createSignal<AspectCitation[]>([]);
  const [shown, setShown] = createSignal<number | null>(null);
  const [diff, setDiff] = createSignal<Diff | null>(null);
  const [take, setTake] = createSignal<Record<number, boolean>>({});
  const [editing, setEditing] = createSignal<number | null>(null);
  const [draft, setDraft] = createSignal("");
  // The backend's own words, verbatim: it knows rules the UI does not.
  const { act, error, busy } = createAction();

  const loadCandidates = () =>
    act(async () => {
      const list = ((await RecordService.ListCandidates()) ?? []) as Candidate[];
      setCandidates(list);
      if (!selected() && list.length > 0) setSelected(list[0].id);
      if (selected()) await refresh();
    });

  const refresh = async () => {
    const id = selected();
    if (!id) return;
    setProfileVersion(((await CandidateProfileService.InUse(id)) ?? null) as Profile | null);
    setReadiness(((await CandidateProfileService.Readiness(id)) ?? null) as Readiness | null);
    const p = profileVersion();
    setCitations(p ? (((await CandidateProfileService.Citations(p.id)) ?? []) as AspectCitation[]) : []);
  };

  onMount(() => void loadCandidates());

  const choose = (id: number) =>
    act(async () => {
      setSelected(id);
      setDiff(null);
      setShown(null);
      setEditing(null);
      await refresh();
    });

  const classify = () =>
    act(async () => {
      const before = profileVersion();
      const made = (await CandidateProfileService.Classify(selected())) as Profile;
      await refresh();
      // A reclassification against an approved version is a proposal, not an
      // update — so show the difference rather than pretending it applied.
      if (before && readiness()?.ready && made.id !== before.id) {
        setDiff(((await CandidateProfileService.DiffAgainstApproved(selected(), made.id)) ?? null) as Diff | null);
      }
    });

  const approve = () =>
    act(async () => {
      const p = profileVersion();
      if (!p) return;
      await CandidateProfileService.Approve(p.id);
      setDiff(null);
      await refresh();
    });

  const remove = (ordinal: number) =>
    act(async () => {
      await CandidateProfileService.RemoveAspect(selected(), ordinal);
      await refresh();
    });

  const startEdit = (ordinal: number, wording: string) => {
    setEditing(ordinal);
    setDraft(wording);
  };

  const saveEdit = () =>
    act(async () => {
      const ordinal = editing();
      if (ordinal === null) return;
      await CandidateProfileService.EditAspect(selected(), ordinal, draft(), {});
      setEditing(null);
      await refresh();
    });

  // Dropping a resume creates the candidate and the artifact together, or
  // neither. Attaching to the selected candidate is the same call with an id.
  const dropResume = (e: Event & { currentTarget: HTMLInputElement }) => {
    const file = e.currentTarget.files?.[0];
    e.currentTarget.value = "";
    if (!file) return;
    return act(async () => {
      const bytes = new Uint8Array(await file.arrayBuffer());
      let binary = "";
      // Indexed rather than iterated: the tsconfig target predates
      // downlevelIteration, and a resume is small enough that it does not
      // matter which loop reads it.
      for (let i = 0; i < bytes.length; i++) binary += String.fromCharCode(bytes[i]);
      const out = (await CandidateProfileService.DropResume({
        initiativeId: props.initiativeId,
        candidateId: selected(),
        fullName: file.name.replace(/\.[^.]+$/, ""),
        displayName: file.name,
        originalFilename: file.name,
        source: "drag-and-drop",
        dataBase64: btoa(binary),
      })) as { candidate?: { id: number } };
      // The artifacts panel is showing a list this drop just changed.
      bumpWorkspace();
      await loadCandidates();
      if (out?.candidate?.id) await choose(out.candidate.id);
    });
  };

  const resolve = () =>
    act(async () => {
      const d = diff();
      if (!d) return;
      const chosen = Object.entries(take())
        .filter(([, v]) => v)
        .map(([k]) => Number(k));
      await CandidateProfileService.ResolveConflicts(selected(), d.proposedProfileId, chosen);
      setDiff(null);
      setTake({});
      await refresh();
    });

  // "Approved" alone is not enough on screen: a stale approval is still the
  // version in use, and hiding that is exactly the quiet drift the lifecycle
  // exists to prevent.
  const stateLabel = () => {
    const p = profileVersion();
    const r = readiness();
    if (!p) return "no profile yet";
    if (p.state === "failed") return "could not be built";
    if (!r?.ready) return "proposed — not yet approved";
    return r.stale ? "approved, but the evidence has changed since" : "approved";
  };

  const citationsFor = (ordinal: number) => citations().filter((c) => c.ordinal === ordinal);

  return (
    <section class="record-section" aria-label="Candidate profile">
      <h3>Candidate profile</h3>

      <div class="search-bar">
        <select
          aria-label="Candidate"
          value={String(selected())}
          onFocus={() => void loadCandidates()}
          onChange={(e) => choose(Number(e.currentTarget.value))}
        >
          <For each={candidates()} fallback={<option value="0">No candidates yet</option>}>
            {(c) => <option value={String(c.id)}>{c.fullName}</option>}
          </For>
        </select>
        {/* Candidates are created elsewhere in the workspace, so this list has
            to be able to catch up without a reload of the whole app. */}
        <button onClick={loadCandidates} disabled={busy()} aria-label="Reload the candidate list">
          Reload
        </button>
        <button onClick={classify} disabled={busy() || !selected()} aria-label="Build this candidate's profile">
          Build profile
        </button>
        <label class="file-drop">
          Attach a resume
          <input type="file" aria-label="Attach a resume" onChange={dropResume} />
        </label>
        <button
          class="primary"
          onClick={approve}
          disabled={busy() || !profileVersion()}
          aria-label="Approve this profile"
        >
          Approve
        </button>
      </div>

      <p class="muted" aria-label="Profile state">
        {stateLabel()}
      </p>
      <Show when={readiness()?.warning}>
        <p class="shell-note" aria-label="Profile warning">
          {readiness()?.warning}
        </p>
      </Show>
      <Show when={readiness() && !readiness()!.ready}>
        <p class="muted" aria-label="Why this candidate is blocked">
          Search and matching are blocked: {readiness()?.reason}
        </p>
      </Show>

      <Show when={error()}>
        <p class="modal-error">{error()}</p>
      </Show>

      <ul class="record-list" aria-label="Profile aspects">
        <For
          each={profileVersion()?.aspects ?? []}
          fallback={<li class="muted">Nothing here yet — build a profile, or add an aspect by hand.</li>}
        >
          {(aspect, i) => (
            <li class="search-hit">
              <span class="artifact-name">
                {aspect.type}
                <span class="muted">
                  {" "}
                  — {aspect.origin === "recruiter_supplied" ? "Recruiter supplied" : "extracted"}
                </span>
              </span>
              <Show
                when={editing() === i()}
                fallback={
                  <>
                    {/* Untrusted: displayed, never rendered, never acted on. */}
                    <pre aria-label={`Aspect ${i() + 1} wording`}>{aspect.wording}</pre>
                    <button aria-label={`Show the evidence for aspect ${i() + 1}`} onClick={() => setShown(i())}>
                      Evidence
                    </button>
                    <button aria-label={`Edit aspect ${i() + 1}`} onClick={() => startEdit(i(), aspect.wording)}>
                      Edit
                    </button>
                    <button aria-label={`Remove aspect ${i() + 1}`} onClick={() => remove(i())}>
                      Remove
                    </button>
                  </>
                }
              >
                <input
                  aria-label={`Wording for aspect ${i() + 1}`}
                  value={draft()}
                  onInput={(e) => setDraft(e.currentTarget.value)}
                  onKeyDown={(e) => {
                    if (e.key === "Enter") saveEdit();
                    if (e.key === "Escape") setEditing(null);
                  }}
                />
                <button class="primary" aria-label={`Save aspect ${i() + 1}`} onClick={saveEdit}>
                  Save
                </button>
                <button aria-label={`Cancel editing aspect ${i() + 1}`} onClick={() => setEditing(null)}>
                  Cancel
                </button>
              </Show>
              <Show when={shown() === i()}>
                <div class="extraction-view">
                  <h4>
                    Evidence
                    <button aria-label={`Close the evidence for aspect ${i() + 1}`} onClick={() => setShown(null)}>
                      Close
                    </button>
                  </h4>
                  <For each={citationsFor(i())} fallback={<p class="muted">No evidence recorded.</p>}>
                    {(c) => (
                      <>
                        <p class="muted">{c.record ? `Recruiter supplied — ${c.record}` : c.location}</p>
                        <pre aria-label={`Cited text for aspect ${i() + 1}`}>{c.text}</pre>
                      </>
                    )}
                  </For>
                </div>
              </Show>
            </li>
          )}
        </For>
      </ul>

      <Show when={diff()}>
        {(d) => (
          <div class="extraction-view" role="region" aria-label="Proposed changes">
            <h4>
              Proposed changes
              <button aria-label="Close the proposed changes" onClick={() => setDiff(null)}>
                Close
              </button>
            </h4>
            <p class="muted">
              {(d().additions ?? []).length} added, {(d().removals ?? []).length} removed,{" "}
              {(d().conflicts ?? []).length} conflicting. The
              approved profile is unchanged until you apply these.
            </p>

            <ul class="record-list" aria-label="Conflicts">
              <For each={d().conflicts} fallback={<li class="muted">No conflicts.</li>}>
                {(c, i) => (
                  <li class="search-hit">
                    <span class="artifact-name">{c.approved.type}</span>
                    <pre aria-label={`Approved wording for conflict ${i() + 1}`}>{c.approved.wording}</pre>
                    <pre aria-label={`Proposed wording for conflict ${i() + 1}`}>{c.proposed.wording}</pre>
                    <label>
                      <input
                        type="checkbox"
                        aria-label={`Take the proposed wording for conflict ${i() + 1}`}
                        checked={!!take()[i()]}
                        onChange={(e) => setTake({ ...take(), [i()]: e.currentTarget.checked })}
                      />{" "}
                      Take the new wording
                    </label>
                  </li>
                )}
              </For>
            </ul>

            <ul class="record-list" aria-label="Additions">
              <For each={d().additions} fallback={<li class="muted">No additions.</li>}>
                {(a) => (
                  <li class="search-hit">
                    <span class="artifact-name">{a.type}</span>
                    <pre>{a.wording}</pre>
                  </li>
                )}
              </For>
            </ul>

            <ul class="record-list" aria-label="Removals">
              <For each={d().removals} fallback={<li class="muted">No removals.</li>}>
                {(r) => (
                  <li class="search-hit">
                    <span class="artifact-name">{r.type}</span>
                    <pre>{r.wording}</pre>
                  </li>
                )}
              </For>
            </ul>

            <button class="primary" aria-label="Apply the proposed changes" onClick={resolve}>
              Apply as a new version
            </button>
          </div>
        )}
      </Show>
    </section>
  );
}
