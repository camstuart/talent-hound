import { Show } from "solid-js";
import type { QueryPreview } from "../../bindings/camstuart/talent-hound";

// The one screen shape for anything that leaves the machine as a query: the
// exact text, editable, with the two warnings, and a send that is the
// recruiter's alone. Used by role discovery and people sourcing so the rule
// has one implementation rather than one per direction.
//
// The two warnings are deliberately different. Naming a company is a legitimate
// search that discloses where you are looking. Naming a person discloses who
// you are looking for, and that is the thing this path exists to prevent
// happening by accident.
export default function QueryPreviewEditor(props: {
  regionLabel: string;
  fieldLabel: string;
  sendLabel: string;
  cancelLabel: string;
  preview: QueryPreview;
  query: string;
  busy: boolean;
  onEdit: (text: string) => void;
  onSend: () => void;
  onCancel: () => void;
}) {
  return (
    <div class="extraction-view" role="region" aria-label={props.regionLabel}>
      <h4>This is exactly what will be sent</h4>
      <textarea aria-label={props.fieldLabel} rows="3" value={props.query} onInput={(e) => props.onEdit(e.currentTarget.value)} />
      <Show when={props.preview.organizationWarning}>
        <p class="shell-note" aria-label="Organization warning">
          {props.preview.organizationWarning}
        </p>
      </Show>
      <Show when={props.preview.identifierWarning}>
        {/* The serious one: shown as an error rather than a note. */}
        <p class="modal-error" aria-label="Identifier warning">
          {props.preview.identifierWarning}
        </p>
      </Show>
      <button class="primary" aria-label={props.sendLabel} disabled={props.busy} onClick={props.onSend}>
        Send it
      </button>
      <button aria-label={props.cancelLabel} onClick={props.onCancel}>
        Cancel
      </button>
    </div>
  );
}
