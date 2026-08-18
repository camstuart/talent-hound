import { createSignal, For, onMount, Show } from "solid-js";
import { RecordService } from "../../bindings/camstuart/talent-hound";
import { InitiativeType } from "../../bindings/camstuart/talent-hound/internal/models";
import type { Candidate } from "../../bindings/camstuart/talent-hound/internal/models";
import { INITIATIVE_TYPE_LABELS } from "./InitiativeIcon";

interface Props {
  onCreate: (name: string, type: InitiativeType, candidateIDs: number[]) => Promise<void>;
  onCancel: () => void;
}

// A job search initiative has exactly one candidate, so the modal either picks
// an existing one or creates it here: without a candidate the form is a dead end.
const NEW_CANDIDATE = "new";

export default function NewInitiativeModal(props: Props) {
  const [name, setName] = createSignal("");
  const [type, setType] = createSignal<InitiativeType>(InitiativeType.InitiativeTypeJobSearch);
  const [candidates, setCandidates] = createSignal<Candidate[]>([]);
  const [candidateId, setCandidateId] = createSignal(NEW_CANDIDATE);
  const [candidateName, setCandidateName] = createSignal("");
  const [submitting, setSubmitting] = createSignal(false);
  const [error, setError] = createSignal("");

  onMount(async () => setCandidates((await RecordService.ListCandidates()) ?? []));

  const needsCandidate = () => type() === InitiativeType.InitiativeTypeJobSearch;

  const submit = async (e: Event) => {
    e.preventDefault();
    if (!name().trim() || submitting()) return;
    setSubmitting(true);
    setError("");
    try {
      let ids: number[] = [];
      if (needsCandidate()) {
        if (candidateId() === NEW_CANDIDATE) {
          const created = await RecordService.CreateCandidate({ fullName: candidateName().trim() } as Candidate);
          if (!created) throw new Error("the candidate could not be created");
          ids = [created.id];
        } else {
          ids = [Number(candidateId())];
        }
      }
      await props.onCreate(name().trim(), type(), ids);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
      setSubmitting(false);
    }
  };

  const incomplete = () =>
    !name().trim() || (needsCandidate() && candidateId() === NEW_CANDIDATE && !candidateName().trim());

  return (
    <div
      class="modal-overlay"
      onClick={(e) => e.target === e.currentTarget && props.onCancel()}
      onKeyDown={(e) => e.key === "Escape" && props.onCancel()}
    >
      <form class="modal" role="dialog" aria-label="New initiative" onSubmit={submit}>
        <h2>New Initiative</h2>
        <label class="modal-field">
          Name
          <input
            type="text"
            placeholder="Initiative name"
            value={name()}
            onInput={(e) => setName(e.currentTarget.value)}
            ref={(el) => setTimeout(() => el.focus())}
          />
        </label>
        <label class="modal-field">
          Type
          <select
            aria-label="Initiative type"
            value={type()}
            onChange={(e) => setType(e.currentTarget.value as InitiativeType)}
          >
            {Object.entries(INITIATIVE_TYPE_LABELS).map(([value, label]) => (
              <option value={value}>{label}</option>
            ))}
          </select>
        </label>
        <Show when={needsCandidate()}>
          <label class="modal-field">
            Candidate
            <select
              aria-label="Candidate"
              value={candidateId()}
              onChange={(e) => setCandidateId(e.currentTarget.value)}
            >
              <option value={NEW_CANDIDATE}>— new candidate —</option>
              <For each={candidates()}>{(c) => <option value={String(c.id)}>{c.fullName}</option>}</For>
            </select>
          </label>
          <Show when={candidateId() === NEW_CANDIDATE}>
            <label class="modal-field">
              Candidate full name
              <input
                type="text"
                placeholder="Candidate full name"
                value={candidateName()}
                onInput={(e) => setCandidateName(e.currentTarget.value)}
              />
            </label>
          </Show>
        </Show>
        {error() && <p class="modal-error">{error()}</p>}
        <div class="modal-actions">
          <button type="button" onClick={() => props.onCancel()}>
            Cancel
          </button>
          <button type="submit" class="primary" disabled={incomplete() || submitting()}>
            Create
          </button>
        </div>
      </form>
    </div>
  );
}
