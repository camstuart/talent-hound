import { createEffect, createSignal, For, Show } from "solid-js";
import { CredentialService, SetupService } from "../../bindings/camstuart/talent-hound";
import type { SetupStatus } from "../../bindings/camstuart/talent-hound";
import { workspaceRevision } from "../workspaceRevision";

// One list that says what this installation still needs before it works, and
// where to go to fix each thing. Everything here is read from the backend's
// own checks — the wizard's steps, the model states, the credential store —
// so the list cannot say "ready" about something the backend would refuse.

export type CheckState = "ready" | "attention" | "optional";

export type CheckItem = {
  key: string;
  title: string;
  state: CheckState;
  detail: string;
  // The section in Settings that fixes it.
  target: "setup" | "model-roles" | "provider-keys";
};

const ROLE_TITLES: Record<string, string> = {
  embed: "Embedding model",
  classify: "Classification model",
  generate: "Generation model",
};

const MODEL_STATE: Record<string, string> = {
  ready: "ready",
  unassigned: "no model chosen",
  endpoint_unavailable: "Ollama is not running",
  model_missing: "not installed",
  pulling: "downloading now",
  pull_declined: "download declined",
  pull_failed: "download failed",
  timeout: "no answer in time",
};

const TARGET_TITLES: Record<CheckItem["target"], string> = {
  setup: "setup",
  "model-roles": "model roles",
  "provider-keys": "provider keys",
};

// checklist turns the backend's state into rows. Exported so the rule for
// each row is testable without a screen.
export function checklist(
  status: SetupStatus | null,
  credentials: Record<string, boolean | undefined>,
  hasStore = true,
): CheckItem[] {
  const items: CheckItem[] = [];
  const step = (name: string) => status?.steps?.find((s) => s.step === name);
  const stepItem = (key: string, title: string, readyDetail: string): CheckItem => {
    const s = step(key);
    const satisfied = s?.satisfied ?? false;
    return {
      key,
      title,
      state: satisfied ? "ready" : "attention",
      detail: satisfied ? readyDetail : s?.detail || "not set up yet",
      target: "setup",
    };
  };

  items.push(stepItem("data_folder", "Data folder", status?.dataFolder ?? ""));

  // Encryption: ready on an encrypted volume, or by the recruiter's recorded
  // choice — said as such, because it is a choice with consequences.
  const enc = step("encryption");
  items.push({
    key: "encryption",
    title: "Volume encryption",
    state: enc?.satisfied ? "ready" : "attention",
    detail: enc?.satisfied ? status?.warning || `volume ${status?.encryption ?? "unknown"}` : enc?.detail || "not checked yet",
    target: "setup",
  });

  items.push(stepItem("sidecar", "Document reader", "verified"));
  items.push(stepItem("ollama", "Ollama", "reachable"));

  for (const role of ["embed", "classify", "generate"]) {
    const m = status?.models?.find((x) => x.role === role);
    const ready = m?.installed && m.state === "ready";
    items.push({
      key: `model:${role}`,
      title: ROLE_TITLES[role] ?? role,
      state: ready ? "ready" : "attention",
      detail: m ? `${m.model} — ${MODEL_STATE[m.state] ?? m.state}` : "no model chosen",
      target: "model-roles",
    });
  }

  items.push({
    key: "exa",
    title: "Exa API key",
    state: credentials.exa ? "ready" : "attention",
    detail: credentials.exa ? "stored" : hasStore ? "needed to find roles and people — no key stored" : "key storage is unavailable on this platform",
    target: "provider-keys",
  });
  items.push({
    key: "github",
    title: "GitHub token",
    state: credentials.github ? "ready" : "optional",
    detail: credentials.github ? "stored" : "optional — enriches candidates from their public GitHub footprint",
    target: "provider-keys",
  });

  items.push(stepItem("acknowledgement", "Data handling acknowledged", "accepted"));
  return items;
}

export default function SystemCheck() {
  const [status, setStatus] = createSignal<SetupStatus | null>(null);
  const [credentials, setCredentials] = createSignal<Record<string, boolean | undefined>>({});
  const [error, setError] = createSignal("");

  // Key storage exists only where the runtime has said it does; asking a
  // platform without one is a question with no answer, and the row says so.
  const os = document.documentElement.dataset.os ?? "";
  const hasStore = os === "windows" || os === "darwin";

  const reload = async () => {
    setError("");
    try {
      const [st, creds] = await Promise.all([
        SetupService.State(),
        hasStore ? CredentialService.List().catch(() => ({})) : Promise.resolve({}),
      ]);
      setStatus((st ?? null) as SetupStatus | null);
      setCredentials((creds ?? {}) as Record<string, boolean | undefined>);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    }
  };

  createEffect(() => {
    workspaceRevision();
    void reload();
  });

  const items = () => checklist(status(), credentials(), hasStore);
  const attention = () => items().filter((i) => i.state === "attention").length;

  // A link to the section that fixes it: same page, so it scrolls and hands
  // focus over rather than navigating anywhere.
  const goTo = (target: CheckItem["target"]) => {
    const el = document.getElementById(target);
    if (!el) return;
    el.scrollIntoView({ block: "start" });
    if (!el.hasAttribute("tabindex")) el.setAttribute("tabindex", "-1");
    el.focus();
  };

  return (
    <section class="record-section" aria-label="System check">
      <h3>System check</h3>
      <p class="muted" aria-label="System check summary">
        <Show when={attention() === 0} fallback={<>{attention()} {attention() === 1 ? "item needs" : "items need"} attention before everything works.</>}>
          Everything this application needs is set up.
        </Show>
      </p>
      <Show when={error()}>
        <p class="modal-error" role="alert">{error()}</p>
      </Show>
      <ul class="record-list" aria-label="System checklist">
        <For each={items()}>
          {(item) => (
            <li class="setting-row" aria-label={`Check ${item.title}`} data-state={item.state}>
              <span class="artifact-name">
                <span aria-hidden="true">{item.state === "ready" ? "✓" : item.state === "optional" ? "○" : "!"}</span>{" "}
                {item.title}
                <span class="muted"> — {item.detail}</span>
              </span>
              <Show when={item.state !== "ready"}>
                <a
                  href={`#${item.target}`}
                  aria-label={`Go to ${TARGET_TITLES[item.target]} for ${item.title}`}
                  onClick={(e) => {
                    e.preventDefault();
                    goTo(item.target);
                  }}
                >
                  Go to {TARGET_TITLES[item.target]}
                </a>
              </Show>
            </li>
          )}
        </For>
      </ul>
      <button aria-label="Check the system again" onClick={() => void reload()}>
        Check again
      </button>
    </section>
  );
}
