import { createAction } from "../act";
import { createSignal, For, onMount, Show } from "solid-js";
import { DiagnosticsService } from "../../bindings/camstuart/talent-hound";
import type { Recovery, Report } from "../../bindings/camstuart/talent-hound";

// Diagnostics are local and built from facts: versions, availability, counts,
// codes. There is nothing here to send anywhere, and no telemetry to turn on.
export default function DiagnosticsPanel() {
  const [report, setReport] = createSignal<Report | null>(null);
  const [recovery, setRecovery] = createSignal<Recovery | null>(null);
  const [logs, setLogs] = createSignal("");
  const [confirmation, setConfirmation] = createSignal("");
  const [deleted, setDeleted] = createSignal("");
  const { act, error, busy } = createAction();

  const load = () =>
    act(async () => {
      setReport((await DiagnosticsService.Diagnostics()) as Report | null);
      setRecovery((await DiagnosticsService.RecoveryProcedure()) as Recovery | null);
      setLogs(await DiagnosticsService.LogsFolder());
    });

  onMount(() => void load());

  return (
    <section class="record-section" aria-label="Diagnostics">
      <h3>Diagnostics</h3>
      <Show when={error()}>
        <p class="modal-error" role="alert">{error()}</p>
      </Show>

      <Show when={report()}>
        {(r) => (
          <div class="extraction-view">
            <p class="muted" aria-label="Application version">
              Talent Hound {r().version} on {r().platform}
            </p>
            <p class="muted" aria-label="Schema version">
              Database schema v{r().schemaVersion} (this build knows v{r().buildSchema})
            </p>
            <p class="muted" aria-label="Data folder">
              Data folder: {r().dataFolder}
            </p>
            <p class="muted" aria-label="Dependency availability">
              Document reader: {r().sidecar} · Ollama: {r().ollama} · Volume: {r().encryption}
            </p>
            <ul class="record-list" aria-label="Record counts">
              <For each={r().counts}>
                {(c) => (
                  <li class="muted">
                    {c.kind}: {c.count}
                  </li>
                )}
              </For>
            </ul>
            <Show when={(r().jobs ?? []).length > 0}>
              <ul class="record-list" aria-label="Job outcomes">
                <For each={r().jobs}>
                  {(j) => (
                    <li class="muted">
                      {j.kind}: {j.count}
                    </li>
                  )}
                </For>
              </ul>
            </Show>
          </div>
        )}
      </Show>

      <button aria-label="Refresh the diagnostic report" disabled={busy()} onClick={() => load()}>
        Refresh
      </button>
      <button
        aria-label="Open the logs folder"
        disabled={busy()}
        onClick={() => act(async () => setLogs(await DiagnosticsService.OpenLogsFolder()))}
      >
        Open logs folder
      </button>
      <Show when={logs()}>
        <p class="muted" aria-label="Logs folder">
          {logs()}
        </p>
      </Show>

      <Show when={recovery()}>
        {(rec) => (
          <div class="extraction-view" aria-label="Recovery procedure">
            <h4>If this machine is lost</h4>
            <ol class="record-list">
              <For each={rec().steps}>{(step) => <li class="muted">{step}</li>}</For>
            </ol>
          </div>
        )}
      </Show>

      {/* Delete-all names the exact folder, and the confirmation is that
          folder: confirming "yes" to a folder described in words is how the
          wrong folder gets deleted. */}
      <div class="extraction-view" aria-label="Delete all data">
        <h4>Delete everything</h4>
        <p class="muted">
          This permanently removes the contents of {report()?.dataFolder}. To confirm, type that folder below.
        </p>
        <input
          aria-label="Folder to confirm"
          value={confirmation()}
          onInput={(e) => setConfirmation(e.currentTarget.value)}
        />
        <button
          aria-label="Delete everything in the data folder"
          disabled={busy()}
          onClick={() =>
            act(async () => {
              const target = await DiagnosticsService.DeleteAll(confirmation());
              setDeleted(`Everything in ${target} was deleted.`);
              setConfirmation("");
            })
          }
        >
          Delete everything
        </button>
        <Show when={deleted()}>
          <p class="muted" aria-label="Deletion outcome">
            {deleted()}
          </p>
        </Show>
      </div>
    </section>
  );
}
