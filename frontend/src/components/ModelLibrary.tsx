import { createSignal, For, Show } from "solid-js";
import { gb } from "./RolePicker";
import type { PickerOption } from "./RolePicker";

// The models on this machine: what is installed, what is downloading, and an
// Add model flow for what is not. Downloads may run concurrently; the roles'
// pickers only ever offer what lands here.
export default function ModelLibrary(props: {
  models: PickerOption[];
  freeDiskBytes: number;
  onPull: (model: string) => void;
}) {
  const [adding, setAdding] = createSignal(false);
  const [custom, setCustom] = createSignal("");

  // The catalog can offer one model for two roles; the library holds it once.
  const unique = () => {
    const seen = new Set<string>();
    return props.models.filter((m) => !seen.has(m.model) && seen.add(m.model));
  };
  const held = () => unique().filter((m) => m.installed || m.pulling);
  const addable = () => unique().filter((m) => !m.installed && !m.pulling);
  const tooBig = (m: PickerOption) => props.freeDiskBytes > 0 && m.approxBytes > props.freeDiskBytes;
  const describe = (m: PickerOption) =>
    m.model + (m.power ? ` — ${m.power}` : "") + (m.approxBytes ? ` · ${gb(m.approxBytes)}` : "");

  return (
    <section class="record-section" aria-label="Model library">
      <h3>Model library</h3>
      <Show when={props.freeDiskBytes > 0}>
        <p class="muted">{gb(props.freeDiskBytes)} free on this disk.</p>
      </Show>
      <ul class="record-list">
        <For each={held()}>
          {(m) => (
            <li class="setting-row">
              <span class="artifact-name">
                {describe(m)}
                <span class="muted"> — {m.pulling ? "downloading now" : "installed"}</span>
              </span>
              <Show when={m.purpose}>
                <span class="muted setting-blurb">{m.purpose}</span>
              </Show>
            </li>
          )}
        </For>
      </ul>
      <Show
        when={adding()}
        fallback={
          <button aria-label="Add model" onClick={() => setAdding(true)}>
            Add model
          </button>
        }
      >
        <ul class="record-list">
          <For each={addable()}>
            {(m) => (
              <li class="setting-row">
                <span class="artifact-name">
                  {describe(m)}
                  <Show when={m.purpose}>
                    <span class="muted"> — {m.purpose}</span>
                  </Show>
                </span>
                <button aria-label={`Download ${m.model}`} disabled={tooBig(m)} onClick={() => props.onPull(m.model)}>
                  Download
                </button>
                <Show when={tooBig(m)}>
                  <span class="muted" role="status">
                    {`${m.model} needs ${gb(m.approxBytes)}, and there is only ${gb(props.freeDiskBytes)} free`}
                  </span>
                </Show>
              </li>
            )}
          </For>
          <li class="setting-row">
            <input
              aria-label="Custom model name"
              placeholder="Model name, e.g. qwen3:8b"
              value={custom()}
              onInput={(e) => setCustom(e.currentTarget.value)}
            />
            <button
              aria-label="Download the custom model"
              disabled={!custom().trim()}
              onClick={() => {
                props.onPull(custom().trim());
                setCustom("");
              }}
            >
              Download
            </button>
          </li>
        </ul>
      </Show>
    </section>
  );
}
