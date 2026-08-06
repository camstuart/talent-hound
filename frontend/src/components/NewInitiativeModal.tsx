import { createSignal } from "solid-js";
import { InitiativeType } from "../../bindings/camstuart/talent-hound/internal/models";
import { INITIATIVE_TYPE_LABELS } from "./InitiativeIcon";

interface Props {
  onCreate: (name: string, type: InitiativeType) => Promise<void>;
  onCancel: () => void;
}

export default function NewInitiativeModal(props: Props) {
  const [name, setName] = createSignal("");
  const [type, setType] = createSignal<InitiativeType>(InitiativeType.InitiativeTypeJobSearch);
  const [submitting, setSubmitting] = createSignal(false);
  const [error, setError] = createSignal("");

  const submit = async (e: Event) => {
    e.preventDefault();
    if (!name().trim() || submitting()) return;
    setSubmitting(true);
    setError("");
    try {
      await props.onCreate(name().trim(), type());
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
      setSubmitting(false);
    }
  };

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
        {error() && <p class="modal-error">{error()}</p>}
        <div class="modal-actions">
          <button type="button" onClick={() => props.onCancel()}>
            Cancel
          </button>
          <button type="submit" class="primary" disabled={!name().trim() || submitting()}>
            Create
          </button>
        </div>
      </form>
    </div>
  );
}
