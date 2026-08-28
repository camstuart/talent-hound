import { createAction } from "../act";
import { createEffect, createSignal, For, onCleanup, onMount, Show } from "solid-js";
import { ModelService, SetupService } from "../../bindings/camstuart/talent-hound";
import type { SetupStatus } from "../../bindings/camstuart/talent-hound";
import { Scope } from "../../bindings/camstuart/talent-hound/internal/setup";
import { bumpWorkspace, workspaceRevision } from "../workspaceRevision";
import RolePicker, { gb } from "./RolePicker";
import type { PickerOption } from "./RolePicker";

// Setup is ordered, and each step blocks the ones after it. The position is not
// stored anywhere: the backend recomputes it, so cancelling is simply not
// finishing a step and resuming is re-entering.

const STEP_TITLES: Record<string, string> = {
  data_folder: "Choose the data folder",
  encryption: "Check the volume's encryption",
  sidecar: "Verify the document reader",
  ollama: "Verify Ollama",
  models: "Install the required models",
  acknowledgement: "Acknowledge how this data is handled",
  first_initiative: "Create the first initiative",
};

export default function FirstRunWizard() {
  const [status, setStatus] = createSignal<SetupStatus | null>(null);
  const [options, setOptions] = createSignal<PickerOption[]>([]);
  const [freeDisk, setFreeDisk] = createSignal(0);
  const [folder, setFolder] = createSignal("");
  const [terms, setTerms] = createSignal<string[]>([]);
  const { act, reloader, error, busy } = createAction();

  const reload = reloader(async (isCurrent) => {
    const [state, terms, opts] = await Promise.all([
      SetupService.State(),
      SetupService.Acknowledgements(),
      ModelService.Options(),
    ]);
    if (!isCurrent()) return;
    setOptions((opts?.models ?? []) as PickerOption[]);
    setFreeDisk(opts?.freeDiskBytes ?? 0);
    const st = state as SetupStatus | null;
    setStatus(st);
    setTerms(terms ?? []);
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
  const pulling = () => (status()?.models ?? []).some((m) => m.state === "pulling");
  // While a download runs, keep asking so the step unlocks when it finishes.
  onMount(() => {
    const poll = setInterval(() => {
      if (pulling()) void reload();
    }, 3000);
    onCleanup(() => clearInterval(poll));
  });

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
              {/* Choosing a folder records where the next launch opens the
                  database. Until then the records go where they are already
                  going, and the encryption state below is about that folder —
                  saying so beats showing one folder and another's answer. */}
              <Show when={st().folderInUse && st().folderInUse !== st().dataFolder}>
                <p class="muted" aria-label="Folder in use">
                  Still using {st().folderInUse} until Talent Hound is restarted.
                </p>
              </Show>
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
              {/* The choice most recruiters will make, said back to them for as
                  long as it is in force. It is a recorded setting, not a bypass. */}
              <Show when={st().warning}>
                <p class="shell-note" aria-label="Unencrypted storage warning">
                  {st().warning}
                </p>
              </Show>
              <Show when={st().scope !== Scope.ScopeDemo && st().encryption !== "encrypted" && !st().unencryptedAccepted}>
                <button
                  aria-label="Store candidate data without disk encryption"
                  disabled={busy()}
                  onClick={() => run(() => SetupService.AcceptUnencrypted(true))}
                >
                  Continue without disk encryption
                </button>
              </Show>
              <Show when={st().unencryptedAccepted}>
                <button
                  aria-label="Require disk encryption again"
                  disabled={busy()}
                  onClick={() => run(() => SetupService.AcceptUnencrypted(false))}
                >
                  Require encryption again
                </button>
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

            <Show when={freeDisk() > 0}>
              <p class="muted">{gb(freeDisk())} free on this disk.</p>
            </Show>
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
                    <RolePicker
                      role={model.role}
                      options={options()}
                      current={model.model}
                      onSelect={(name) => run(() => ModelService.Assign({ role: model.role, endpoint: "", model: name, digest: "", params: "" }))}
                    />
                    <Show when={!model.installed}>
                      <button
                        aria-label={`Download the ${model.role} model`}
                        disabled={busy() || pulling()}
                        onClick={() => run(() => SetupService.PullModel(model.role))}
                      >
                        Download
                      </button>
                      <button
                        aria-label={`Skip the ${model.role} model for now`}
                        disabled={busy() || pulling()}
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
