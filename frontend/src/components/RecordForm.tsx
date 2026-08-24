import { createSignal, For, Show } from "solid-js";

export interface FieldSpec {
  key: string;
  label: string;
  type?: "text" | "date" | "number" | "select";
  options?: { value: string; label: string }[];
  required?: boolean;
  placeholder?: string;
  // A lowercase fragment of the backend's error message that identifies this
  // field, so a rejection lands under the input that caused it instead of in a
  // generic banner. The backend is the only authority on what is valid.
  match?: string;
}

interface Props {
  legend: string;
  fields: FieldSpec[];
  submitLabel: string;
  initial?: Record<string, string>;
  onSubmit: (values: Record<string, string>) => Promise<void>;
}

const initial = (fields: FieldSpec[]): Record<string, string> =>
  Object.fromEntries(fields.map((f) => [f.key, f.type === "select" ? (f.options?.[0]?.value ?? "") : ""]));

// One form for every record type: the fields differ, the behaviour does not.
export default function RecordForm(props: Props) {
  const [values, setValues] = createSignal({ ...initial(props.fields), ...(props.initial ?? {}) });
  const [errors, setErrors] = createSignal<Record<string, string>>({});
  const [formError, setFormError] = createSignal("");
  const [busy, setBusy] = createSignal(false);

  const set = (key: string, value: string) => setValues({ ...values(), [key]: value });

  const submit = async (e: Event) => {
    e.preventDefault();
    if (busy()) return;
    setFormError("");

    const missing = props.fields.filter((f) => f.required && !values()[f.key].trim());
    if (missing.length > 0) {
      setErrors(Object.fromEntries(missing.map((f) => [f.key, `${f.label} is required`])));
      return;
    }
    setErrors({});

    setBusy(true);
    try {
      await props.onSubmit(values());
      setValues({ ...initial(props.fields), ...(props.initial ?? {}) });
    } catch (err) {
      // Show the backend's own words: it knows rules the form does not.
      const message = err instanceof Error ? err.message : String(err);
      const owner = props.fields.find((f) => f.match && message.toLowerCase().includes(f.match));
      if (owner) setErrors({ [owner.key]: message });
      else setFormError(message);
    } finally {
      setBusy(false);
    }
  };

  return (
    <form class="record-form" aria-label={props.legend} onSubmit={submit}>
      <h4>{props.legend}</h4>
      <div class="record-fields">
        <For each={props.fields}>
          {(field) => (
            <div class="record-field">
              {/* The error sits outside the label so it never becomes part of
                  the field's accessible name. */}
              <label>
                <span>
                  {field.label}
                  {field.required ? " *" : ""}
                </span>
                <Show
                  when={field.type === "select"}
                  fallback={
                    <input
                      type={field.type === "date" ? "date" : field.type === "number" ? "number" : "text"}
                      placeholder={field.placeholder}
                      aria-invalid={errors()[field.key] ? "true" : undefined}
                      value={values()[field.key]}
                      onInput={(e) => set(field.key, e.currentTarget.value)}
                    />
                  }
                >
                  <select
                    aria-invalid={errors()[field.key] ? "true" : undefined}
                    value={values()[field.key]}
                    onChange={(e) => set(field.key, e.currentTarget.value)}
                  >
                    <For each={field.options}>{(o) => <option value={o.value}>{o.label}</option>}</For>
                  </select>
                </Show>
              </label>
              <Show when={errors()[field.key]}>
                <span class="field-error">{errors()[field.key]}</span>
              </Show>
            </div>
          )}
        </For>
      </div>
      <Show when={formError()}>
        <p class="modal-error" role="alert">{formError()}</p>
      </Show>
      <div class="record-form-actions">
        <button type="submit" class="primary" disabled={busy()}>
          {props.submitLabel}
        </button>
      </div>
    </form>
  );
}

// num turns a form value into the optional integer the backend expects.
export const num = (v: string): number | null => (v.trim() === "" ? null : Number(v));

// list splits a comma-separated form value; the backend trims and drops blanks.
export const list = (v: string): string[] => v.split(",");
