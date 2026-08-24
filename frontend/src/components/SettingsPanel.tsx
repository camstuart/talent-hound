import { createSignal, For, onCleanup, onMount, Show } from "solid-js";
import { CredentialService, ModelService } from "../../bindings/camstuart/talent-hound";
import { ModelRole } from "../../bindings/camstuart/talent-hound/internal/models";
import type { Status } from "../../bindings/camstuart/talent-hound";
import FirstRunWizard from "./FirstRunWizard";
import ModelPicker, { gb } from "./ModelPicker";
import type { PickerOption } from "./ModelPicker";
import DiagnosticsPanel from "./DiagnosticsPanel";

// What each availability state means, in the recruiter's terms. They are
// separate states because the thing to do about each one is different.
const STATE_LABELS: Record<string, string> = {
  ready: "ready",
  unassigned: "no model chosen",
  endpoint_unavailable: "Ollama is not running",
  model_missing: "not installed",
  pulling: "downloading now",
  pull_declined: "download declined",
  pull_failed: "download failed",
  timeout: "no answer in time",
  malformed_response: "unexpected answer",
  out_of_memory: "not enough memory",
};

const ROLE_BLURBS: Record<string, string> = {
  [ModelRole.RoleEmbed]: "Embeds evidence chunks and profile aspects.",
  [ModelRole.RoleClassify]: "Decomposes profiles and flags prohibited criteria.",
  [ModelRole.RoleGenerate]: "Writes assessments, summaries, drafts, and chat.",
};

const PROVIDER_LABELS: Record<string, string> = {
  exa: "Exa (role discovery)",
  cloud: "Cloud provider (optional overrides)",
};

// The three local model roles and the provider keys. Everything here is local
// configuration: no candidate content passes through this panel.
export default function SettingsPanel() {
  // main.tsx renders only after the runtime has answered data-os, so an absent
  // value means "no runtime" — never assume Windows.
  const os = document.documentElement.dataset.os ?? "";
  const credentialStore = os === "windows" ? "Windows Credential Manager" : os === "darwin" ? "macOS Keychain" : "";
  const osName = os === "darwin" ? "macOS" : os === "linux" ? "Linux" : os === "windows" ? "Windows" : "this platform";
  const [statuses, setStatuses] = createSignal<Status[]>([]);
  const [options, setOptions] = createSignal<PickerOption[]>([]);
  const [freeDisk, setFreeDisk] = createSignal(0);
  const [assignments, setAssignments] = createSignal<Record<string, { model: string; revision: number; validation: string }>>({});
  const [inherited, setInherited] = createSignal<Record<string, boolean>>({});
  const [credentials, setCredentials] = createSignal<Record<string, boolean | undefined>>({});
  const [keys, setKeys] = createSignal<Record<string, string>>({});
  const [error, setError] = createSignal("");

  const act = async (run: () => Promise<unknown>) => {
    setError("");
    try {
      await run();
      await reload();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    }
  };

  const reload = async () => {
    const registry = (await ModelService.Registry()) ?? [];
    const assigned: Record<string, { model: string; revision: number; validation: string }> = {};
    const inheritedRoles: Record<string, boolean> = {};
    for (const res of registry) {
      inheritedRoles[res.role] = res.inherited;
      if (res.assignment) {
        assigned[res.role] = {
          model: res.assignment.model,
          revision: res.assignment.revision,
          validation: res.assignment.validation,
        };
      }
    }
    setAssignments(assigned);
    setInherited(inheritedRoles);
    setStatuses(((await ModelService.Check()) ?? []) as Status[]);
    const opts = await ModelService.Options();
    setOptions((opts?.models ?? []) as PickerOption[]);
    setFreeDisk(opts?.freeDiskBytes ?? 0);
    if (credentialStore) setCredentials((await CredentialService.List()) ?? {});
  };
  onMount(() => void act(reload));
  // A download underway in any window shows up in the statuses; while one is,
  // keep asking so the picker unlocks the moment it finishes.
  onMount(() => {
    const poll = setInterval(() => {
      if (pulling()) void reload();
    }, 3000);
    onCleanup(() => clearInterval(poll));
  });

  const statusOf = (role: ModelRole) => statuses().find((s) => s.role === role);
  const pulling = () => statuses().some((s) => s.state === "pulling");
  const label = (state: string | undefined) => (state ? (STATE_LABELS[state] ?? state) : "unknown");

  const assign = (role: ModelRole, model: string) =>
    act(() => ModelService.Assign({ role, endpoint: "", model: model.trim(), digest: "", params: "" }));

  const storeKey = (provider: string) =>
    act(async () => {
      await CredentialService.Store(provider, keys()[provider] ?? "");
      // Cleared immediately: the value has gone where it belongs and this
      // application never asks for it back.
      setKeys((k) => ({ ...k, [provider]: "" }));
    });

  return (
    <div class="settings">
      <section class="record-section" aria-label="Model roles">
        <h3>Model roles</h3>
        <p class="muted">All three roles run locally. Candidate content is never sent anywhere else.</p>
        <Show when={freeDisk() > 0}>
          <p class="muted">{gb(freeDisk())} free on this disk.</p>
        </Show>
        <ul class="record-list">
          <For each={Object.values(ModelRole)}>
            {(role) => (
              <li class="setting-row">
                <span class="artifact-name">
                  {role}
                  <span class="muted">
                    {" "}
                    — {assignments()[role]?.model ?? "no model"}
                    <Show when={assignments()[role]}>
                      {(a) => ` (revision ${a().revision}, ${a().validation})`}
                    </Show>
                    <Show when={inherited()[role]}> — inherited from generate</Show>
                    , {label(statusOf(role)?.state)}
                  </span>
                </span>
                <span class="muted setting-blurb">{ROLE_BLURBS[role]}</span>
                <ModelPicker
                  role={role}
                  options={options().filter((o) => o.role === role)}
                  current={assignments()[role]?.model ?? ""}
                  freeDiskBytes={freeDisk()}
                  busy={pulling()}
                  onAssign={(model) => assign(role, model)}
                />
                <Show when={statusOf(role)?.state === "model_missing" || statusOf(role)?.state === "pull_failed"}>
                  <button aria-label={`Download the ${role} model`} onClick={() => act(() => ModelService.Pull(role))}>
                    Download
                  </button>
                  <button aria-label={`Decline the ${role} download`} onClick={() => act(() => ModelService.Decline(role))}>
                    Not now
                  </button>
                </Show>
              </li>
            )}
          </For>
        </ul>
      </section>

      <section class="record-section" aria-label="Provider keys">
        <h3>Provider keys</h3>
        <p class="muted">
          <Show when={credentialStore} fallback={<>Provider key storage is unavailable on {osName}.</>}>
            Keys are held by {credentialStore}. They are never stored in this application's data, and there is no way to
            read one back.
          </Show>
        </p>
        <ul class="record-list">
          <For each={Object.keys(PROVIDER_LABELS)}>
            {(provider) => (
              <li class="setting-row">
                <span class="artifact-name">
                  {PROVIDER_LABELS[provider]}
                  <span class="muted">
                    {" "}— {credentialStore ? (credentials()[provider] ? "key stored" : "no key stored") : "unavailable"}
                  </span>
                </span>
                <input
                  type="password"
                  aria-label={`${provider} key`}
                  placeholder="Paste the key"
                  value={keys()[provider] ?? ""}
                  disabled={!credentialStore}
                  onInput={(e) => setKeys((k) => ({ ...k, [provider]: e.currentTarget.value }))}
                />
                <button aria-label={`Save ${provider}`} disabled={!credentialStore} onClick={() => storeKey(provider)}>
                  Save
                </button>
                <Show when={credentialStore && credentials()[provider]}>
                  <button aria-label={`Remove ${provider}`} onClick={() => act(() => CredentialService.Delete(provider))}>
                    Remove
                  </button>
                </Show>
              </li>
            )}
          </For>
        </ul>
      </section>

      <FirstRunWizard />
      <DiagnosticsPanel />

      <Show when={error()}>
        <p class="modal-error" role="alert">{error()}</p>
      </Show>
    </div>
  );
}
