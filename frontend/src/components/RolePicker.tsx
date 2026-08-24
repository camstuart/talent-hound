import { For, Show } from "solid-js";

// One library entry as the backend's ModelService.Options() returns it.
export type PickerOption = {
  role: string;
  model: string;
  purpose: string;
  power: string;
  approxBytes: number;
  installed: boolean;
  pulling: boolean;
};

export const gb = (bytes: number) => `${(bytes / 1024 ** 3).toFixed(1)} GB`;

// The models a role may be assigned: installed, and either made for this role
// or brought in by the recruiter (no role means "offered everywhere").
export const forRole = (options: PickerOption[], role: string) =>
  options.filter((o) => o.installed && (o.role === role || !o.role));

// A role's model choice. Selecting assigns immediately; downloads live in the
// model library, not here.
export default function RolePicker(props: {
  role: string;
  options: PickerOption[];
  current: string;
  onSelect: (model: string) => void;
}) {
  const choices = () => {
    const suitable = forRole(props.options, props.role);
    // The persisted assignment always shows, even if its model has vanished
    // from the endpoint: hiding it would misreport what is configured.
    if (props.current && !suitable.some((o) => o.model === props.current)) {
      return [...suitable, { role: props.role, model: props.current, purpose: "", power: "", approxBytes: 0, installed: false, pulling: false }];
    }
    return suitable;
  };
  const label = (o: PickerOption) =>
    o.model + (o.power ? ` — ${o.power}` : "") + (o.approxBytes ? ` · ${gb(o.approxBytes)}` : "");

  return (
    <select
      aria-label={`Model for ${props.role}`}
      onChange={(e) => e.currentTarget.value && props.onSelect(e.currentTarget.value)}
    >
      <Show when={!props.current}>
        <option value="" disabled selected>
          Choose a model
        </option>
      </Show>
      <For each={choices()}>
        {(o) => (
          <option value={o.model} selected={o.model === props.current}>
            {label(o)}
          </option>
        )}
      </For>
    </select>
  );
}
