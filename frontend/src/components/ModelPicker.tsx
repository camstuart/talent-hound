import { createSignal, For, Show } from "solid-js";

// One catalog entry as the backend's ModelService.Options() returns it.
export type PickerOption = {
  role: string;
  model: string;
  purpose: string;
  power: string;
  approxBytes: number;
  installed: boolean;
};

export const gb = (bytes: number) => `${(bytes / 1024 ** 3).toFixed(1)} GB`;

const CUSTOM = "__custom__";

// A curated model choice for one role. Pure props: the parent owns the
// bindings, so both the settings panel and the first-run wizard can use it.
export default function ModelPicker(props: {
  role: string;
  options: PickerOption[];
  current: string;
  freeDiskBytes: number;
  busy: boolean;
  onAssign: (model: string) => void;
}) {
  const [choice, setChoice] = createSignal("");
  const [custom, setCustom] = createSignal("");

  const selected = () => choice() || props.current;
  const chosenOption = () => props.options.find((o) => o.model === selected());
  // A model already at the endpoint costs no disk; an unknown free-space
  // answer (0) refuses nothing rather than everything.
  const tooBig = () => {
    const o = chosenOption();
    return !!o && !o.installed && props.freeDiskBytes > 0 && o.approxBytes > props.freeDiskBytes;
  };
  const assignName = () => (choice() === CUSTOM ? custom().trim() : selected());

  const label = (o: PickerOption) =>
    `${o.model} — ${o.power} · ${gb(o.approxBytes)}${o.installed ? " · installed" : ""}`;

  return (
    <>
      <select
        aria-label={`Model for ${props.role}`}
        disabled={props.busy}
        value={selected()}
        onChange={(e) => setChoice(e.currentTarget.value)}
      >
        <option value="" disabled>
          Choose a model
        </option>
        <For each={props.options}>
          {(o) => (
            <option value={o.model} classList={{ installed: o.installed }}>
              {label(o)}
            </option>
          )}
        </For>
        <option value={CUSTOM}>Custom…</option>
      </select>
      <Show when={choice() === CUSTOM}>
        <input
          aria-label={`Custom model for ${props.role}`}
          placeholder="Model name, e.g. qwen3:8b"
          value={custom()}
          disabled={props.busy}
          onInput={(e) => setCustom(e.currentTarget.value)}
        />
      </Show>
      <button
        aria-label={`Assign a model to ${props.role}`}
        disabled={props.busy || tooBig() || !assignName()}
        onClick={() => props.onAssign(assignName())}
      >
        Assign
      </button>
      <Show when={tooBig()}>
        <span class="muted" role="status">
          {`${chosenOption()!.model} needs ${gb(chosenOption()!.approxBytes)}, and there is only ${gb(props.freeDiskBytes)} free`}
        </span>
      </Show>
    </>
  );
}
