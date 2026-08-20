import { createAction } from "../act";
import { createEffect, createSignal, For, Show } from "solid-js";
import { DraftService, QAService, RecordService } from "../../bindings/camstuart/talent-hound";
import type { Answer, Candidate, Draft } from "../../bindings/camstuart/talent-hound/internal/models";
import { workspaceRevision } from "../workspaceRevision";

// Ask, draft, edit, copy. The application drafts; the recruiter sends — there
// is no send button here because there is no sender anywhere, and a test proves
// it.
//
// An answer the evidence does not support says so rather than producing a
// plausible paragraph, and a suggestion the assistant makes is a suggestion
// until a person applies it.
export default function DraftsPanel(props: { initiativeId: number }) {
  const [question, setQuestion] = createSignal("");
  const [answers, setAnswers] = createSignal<Answer[]>([]);
  const [shownCitations, setShownCitations] = createSignal<number | null>(null);
  const [candidates, setCandidates] = createSignal<Candidate[]>([]);
  const [candidate, setCandidate] = createSignal(0);
  const [drafts, setDrafts] = createSignal<Draft[]>([]);
  const [editing, setEditing] = createSignal<number | null>(null);
  const [draftBody, setDraftBody] = createSignal("");
  const [copied, setCopied] = createSignal<number | null>(null);
  // The backend's own words, verbatim: it knows rules the UI does not.
  const { act, error, busy } = createAction();

  const reload = () =>
    act(async () => {
      setAnswers(((await QAService.Answers(props.initiativeId)) ?? []) as Answer[]);
      setDrafts(((await DraftService.Drafts(props.initiativeId)) ?? []) as Draft[]);
      setCandidates(((await RecordService.ListCandidates()) ?? []) as Candidate[]);
    });

  createEffect(() => {
    workspaceRevision();
    void reload();
  });

  const ask = () =>
    act(async () => {
      const text = question().trim();
      if (!text) return;
      await QAService.Ask(props.initiativeId, text);
      setQuestion("");
      await reload();
    });

  const generate = (kind: string) =>
    act(async () => {
      await DraftService.Generate({
        initiativeId: props.initiativeId,
        candidateId: candidate(),
        roleId: 0,
        kind,
      } as never);
      await reload();
    });

  const saveEdit = (id: number) =>
    act(async () => {
      await DraftService.Edit(id, "", draftBody());
      setEditing(null);
      await reload();
    });

  // Copying puts the text on the clipboard and records that it happened. The
  // event carries no draft text — the audit log is the artifact most likely to
  // be exported.
  const copy = (draft: Draft) =>
    act(async () => {
      await navigator.clipboard?.writeText?.(draft.body).catch(() => undefined);
      await DraftService.Copy(draft.id);
      setCopied(draft.id);
      await reload();
    });

  const discard = (id: number) =>
    act(async () => {
      await DraftService.Discard(id);
      await reload();
    });

  const citationsOf = (answer: Answer): { ref: string; text: string; location: string }[] => {
    try {
      return JSON.parse(answer.citations ?? "[]");
    } catch {
      return [];
    }
  };

  const claimsOf = (draft: Draft): { text: string; refs: string[] }[] => {
    try {
      return JSON.parse(draft.claims ?? "[]");
    } catch {
      return [];
    }
  };

  return (
    <section class="record-section" aria-label="Ask and draft">
      <h3>Ask and draft</h3>
      <p class="muted">
        Answers come only from this initiative's approved evidence, and cite it. Drafts are yours to edit and send —
        this application cannot send anything.
      </p>

      <div class="search-bar">
        <input
          aria-label="Question"
          placeholder="Ask about this workspace"
          value={question()}
          onInput={(e) => setQuestion(e.currentTarget.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") ask();
          }}
        />
        <button class="primary" aria-label="Ask this question" disabled={busy()} onClick={ask}>
          Ask
        </button>
      </div>

      <Show when={error()}>
        <p class="modal-error" role="alert">{error()}</p>
      </Show>

      <ul class="record-list" aria-label="Answers">
        <For each={answers()} fallback={<li class="muted">No questions asked yet.</li>}>
          {(a, i) => (
            <li class="search-hit">
              <span class="artifact-name">
                {a.question}
                <span class="muted"> — {a.supported ? "supported by evidence" : "not supported"}</span>
              </span>
              {/* Untrusted: displayed, never rendered. */}
              <pre data-provenance="ai" aria-label={`Answer ${i() + 1}`}>{a.answer}</pre>
              <Show when={citationsOf(a).length > 0}>
                <button
                  aria-label={`Show the evidence for answer ${i() + 1}`}
                  onClick={() => setShownCitations(shownCitations() === i() ? null : i())}
                >
                  Evidence
                </button>
              </Show>
              <Show when={shownCitations() === i()}>
                <For each={citationsOf(a)}>
                  {(c) => (
                    <>
                      <p class="muted">{c.location}</p>
                      <pre data-provenance="source" aria-label={`Cited evidence for answer ${i() + 1}`}>{c.text}</pre>
                    </>
                  )}
                </For>
              </Show>
              <Show when={(a.proposals ?? []).length > 0}>
                {/* Suggestions, not changes. Applying one is a separate act in
                    the criteria panel, with the same refusals as typing it. */}
                <p class="shell-note" aria-label={`Suggestions from answer ${i() + 1}`}>
                  Suggested criteria (add them yourself in Search criteria): {(a.proposals ?? []).join("; ")}
                </p>
              </Show>
            </li>
          )}
        </For>
      </ul>

      <div class="search-bar">
        <select
          aria-label="Draft about candidate"
          value={String(candidate())}
          onFocus={() => void reload()}
          onChange={(e) => setCandidate(Number(e.currentTarget.value))}
        >
          <option value="0">Choose a candidate</option>
          <For each={candidates()}>{(c) => <option value={String(c.id)}>{c.fullName}</option>}</For>
        </select>
        <button aria-label="Write a pitch" disabled={busy() || !candidate()} onClick={() => generate("pitch")}>
          Write a pitch
        </button>
        <button
          aria-label="Write an outreach message"
          disabled={busy() || !candidate()}
          onClick={() => generate("outreach")}
        >
          Write outreach
        </button>
      </div>

      <ul class="record-list" aria-label="Drafts">
        <For each={drafts()} fallback={<li class="muted">No drafts yet.</li>}>
          {(d, i) => (
            <li class="search-hit">
              <span class="artifact-name">
                {d.kind}
                <span class="muted">
                  {" "}
                  — {d.state}, copied {d.copies} {d.copies === 1 ? "time" : "times"}
                </span>
              </span>
              <Show
                when={editing() === d.id}
                fallback={
                  <>
                    {/* Untrusted: displayed, never rendered. */}
                    <pre data-provenance="ai" aria-label={`Draft ${i() + 1}`}>{d.body}</pre>
                    <Show when={d.state === "active"}>
                      <button
                        aria-label={`Edit draft ${i() + 1}`}
                        onClick={() => {
                          setEditing(d.id);
                          setDraftBody(d.body);
                        }}
                      >
                        Edit
                      </button>
                      <button aria-label={`Copy draft ${i() + 1}`} onClick={() => copy(d)}>
                        Copy
                      </button>
                      <button aria-label={`Discard draft ${i() + 1}`} onClick={() => discard(d.id)}>
                        Discard
                      </button>
                    </Show>
                    <Show when={copied() === d.id}>
                      <span class="muted" aria-label={`Copy confirmation for draft ${i() + 1}`}>
                        {" "}
                        Copied — paste it wherever you send from.
                      </span>
                    </Show>
                  </>
                }
              >
                <textarea
                  aria-label={`Text of draft ${i() + 1}`}
                  rows="6"
                  value={draftBody()}
                  onInput={(e) => setDraftBody(e.currentTarget.value)}
                  onKeyDown={(e) => {
                    if (e.key === "Escape") setEditing(null);
                  }}
                />
                <button class="primary" aria-label={`Save draft ${i() + 1}`} onClick={() => saveEdit(d.id)}>
                  Save
                </button>
                <button aria-label={`Cancel editing draft ${i() + 1}`} onClick={() => setEditing(null)}>
                  Cancel
                </button>
              </Show>
              <Show when={claimsOf(d).length > 0}>
                <ul aria-label={`What draft ${i() + 1} rests on`}>
                  <For each={claimsOf(d)}>
                    {(c) => (
                      <li class="muted">
                        “{c.text}” — from {c.refs.join(", ")}
                      </li>
                    )}
                  </For>
                </ul>
              </Show>
            </li>
          )}
        </For>
      </ul>
    </section>
  );
}
