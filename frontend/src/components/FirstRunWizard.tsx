import { createAction } from "../act";
import { createEffect, createSignal, For, Show } from "solid-js";
import { SetupService } from "../../bindings/camstuart/talent-hound";
import type { SetupStatus } from "../../bindings/camstuart/talent-hound";
import { Scope } from "../../bindings/camstuart/talent-hound/internal/setup";
import { bumpWorkspace, workspaceRevision } from "../workspaceRevision";

// Setup is ordered, and each step blocks the ones after it. The position is not
// stored anywhere: the backend recomputes it, so cancelling is simply not
// finishing a step and resuming is re-entering.

const STEP_TITLES: Record<string, string> = {
  data_folder: "Choose the data folder",
  encryption: "Verify the volume is encrypted",
  sidecar: "Verify the document reader",
  ollama: "Verify Ollama",
  models: "Install the required models",
  acknowledgement: "Acknowledge how this data is handled",
  first_initiative: "Create the first initiative",
};

const gb = (bytes: number) => `${(bytes / 1024 / 1024 / 1024).toFixed(1)} GB`;

export default function FirstRunWizard() {
  const [status, setStatus] = createSignal<SetupStatus | null>(null);
  const [folder, setFolder] = createSignal("");
  const [terms, setTerms] = createSignal<string[]>([]);
  const { act, error, busy } = createAction();

  const reload = () =>
    act(async () => {
      const st = (await SetupService.State()) as SetupStatus | null;
      setStatus(st);
      setTerms((await SetupService.Acknowledgements()) ?? []);
      if (st && !folder()) setFolder(st.dataFolder);
    });

  createEffect(() => {
    workspaceRevision();
    void reload();
  });

  const run = (fn: () => Promise<unknown>) =>
    act(async () => {
      await fn();
      bumpWorkspace();
      await reload();
    });

  const current = () => status()?.next ?? "";

  return (
    <section class="record-section" aria-label="Setup">
      <h3>Setup</h3>
      <Show when={status()}>
        {(st) => (
          <>
            <p class="muted" aria-label="Setup position">
              {st().complete
                ? "Setup is complete."
                : `Next: ${STEP_TITLES[st().next] ?? st().next}`}
            </p>
            <p class="muted" aria-label="Application version">
              Version {st().version}
            </p>

            <Show when={error()}>
              <p class="modal-error" role="alert">{error()}</p>
            </Show>

            <ol class="record-list" aria-label="Setup steps">
              <For each={st().steps}>
                {(step) => (
                  <li class="search-hit" data-satisfied={step.satisfied ? "yes" : "no"}>
                    <span class="artifact-name">{STEP_TITLES[step.step] ?? step.step}</span>
                    <span class="muted">
                      {step.satisfied ? "done" : step.step === st().next ? "now" : "not reached"}
                    </span>
                    <Show when={step.detail}>
                      <span class="shell-note" aria-label={`Why ${step.step} is not done`}>
                        {step.detail}
                      </span>
                    </Show>
                  </li>
                )}
              </For>
            </ol>

            {/* The data folder: the one folder that holds everything, and the
                one folder recovery copies. */}
            <Show when={current() === "data_folder" || st().complete}>
              <div class="extraction-view">
                <label>
                  Data folder
                  <input
                    aria-label="Data folder"
                    value={folder()}
                    onInput={(e) => setFolder(e.currentTarget.value)}
                  />
                </label>
                <button
                  aria-label="Use this data folder"
                  disabled={busy()}
                  onClick={() => run(() => SetupService.ChooseFolder(folder()))}
                >
                  Use this folder
                </button>
              </div>
            </Show>

            {/* Real data needs an encrypted volume. Demo is a deliberate
                choice, never something the application switches to quietly. */}
            <div class="extraction-view">
              <p class="muted" aria-label="Data scope">
                Scope: {st().scope === Scope.ScopeDemo ? "demo — this installation holds no candidate data" : "real"}
              </p>
              <p class="muted" aria-label="Encryption state">
                Volume: {st().encryption}
              </p>
              <Show when={!st().realData}>
                <p class="modal-error" aria-label="Why real data is blocked">
                  {st().realDataWhy}
                </p>
              </Show>
              <button
                aria-label="Check the volume again"
                disabled={busy()}
                onClick={() => run(() => SetupService.Recheck())}
              >
                Check again
              </button>
              <button
                aria-label="Work in demo scope"
                disabled={busy()}
                onClick={() => run(() => SetupService.SetScope(Scope.ScopeDemo))}
              >
                Use demo scope
              </button>
              <button
                aria-label="Work with real candidate data"
                disabled={busy()}
                onClick={() => run(() => SetupService.SetScope(Scope.ScopeReal))}
              >
                Use real scope
              </button>
            </div>

            <ul class="record-list" aria-label="Required models">
              <For each={st().models}>
                {(model) => (
                  <li class="search-hit">
                    <span class="artifact-name">
                      {model.role}: {model.model}
                    </span>
                    <span class="muted">
                      {gb(model.approxBytes)} — {model.installed ? "installed" : model.state || "missing"}
                    </span>
                    <Show when={!model.installed}>
                      <button
                        aria-label={`Download the ${model.role} model`}
                        disabled={busy()}
                        onClick={() => run(() => SetupService.PullModel(model.role))}
                      >
                        Download
                      </button>
                      <button
                        aria-label={`Skip the ${model.role} model for now`}
                        disabled={busy()}
                        onClick={() => run(() => SetupService.DeclineModel(model.role))}
                      >
                        Not now
                      </button>
                    </Show>
                  </li>
                )}
              </For>
            </ul>

            <Show when={!st().acknowledged}>
              <div class="extraction-view" aria-label="Data handling">
                <ul class="record-list">
                  <For each={terms()}>{(term) => <li class="muted">{term}</li>}</For>
                </ul>
                <button
                  class="primary"
                  aria-label="Acknowledge these responsibilities"
                  disabled={busy()}
                  onClick={() => run(() => SetupService.Acknowledge())}
                >
                  I acknowledge these
                </button>
              </div>
            </Show>
          </>
        )}
      </Show>
    </section>
  );
}
