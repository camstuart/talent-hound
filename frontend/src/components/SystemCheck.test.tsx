import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@solidjs/testing-library";
import SystemCheck, { checklist } from "./SystemCheck";

// The Go backend is not running: bindings are mocked. Fixtures are invented.
const { state, setupMocks, credentialMocks } = vi.hoisted(() => {
  const state = {
    status: null as Record<string, unknown> | null,
    credentials: {} as Record<string, boolean>,
  };
  return {
    state,
    setupMocks: { State: vi.fn(async () => state.status) },
    credentialMocks: { List: vi.fn(async () => state.credentials) },
  };
});

vi.mock("../../bindings/camstuart/talent-hound", () => ({
  SetupService: setupMocks,
  CredentialService: credentialMocks,
}));

const STEPS = ["data_folder", "encryption", "sidecar", "ollama", "models", "acknowledgement", "first_initiative"];

const aStatus = (unsatisfied: string[] = [], over: Record<string, unknown> = {}) => ({
  next: unsatisfied[0] ?? "complete",
  complete: unsatisfied.length === 0,
  steps: STEPS.map((step) => ({
    step,
    satisfied: !unsatisfied.includes(step),
    detail: unsatisfied.includes(step) ? `${step} is not done` : "",
  })),
  models: [
    { role: "embed", model: "nomic-embed-text", approxBytes: 1, installed: true, state: "ready" },
    { role: "classify", model: "qwen2.5:7b-instruct", approxBytes: 1, installed: true, state: "ready" },
    { role: "generate", model: "qwen2.5:7b-instruct", approxBytes: 1, installed: false, state: "model_missing" },
  ],
  dataFolder: "C:\\Talent Hound",
  encryption: "encrypted",
  realData: true,
  realDataWhy: "",
  unencryptedAccepted: false,
  warning: "",
  ...over,
});

beforeEach(() => {
  document.documentElement.dataset.os = "windows";
  state.status = aStatus();
  state.credentials = { exa: true, cloud: false, github: false };
  vi.clearAllMocks();
});

describe("checklist", () => {
  it("marks each thing the backend says is missing, and nothing it says is present", () => {
    const items = checklist(aStatus(["sidecar"]) as never, { exa: false });
    const byKey = Object.fromEntries(items.map((i) => [i.key, i]));
    expect(byKey.sidecar.state).toBe("attention");
    expect(byKey.sidecar.detail).toBe("sidecar is not done");
    expect(byKey.ollama.state).toBe("ready");
    expect(byKey["model:embed"].state).toBe("ready");
    expect(byKey["model:generate"].state).toBe("attention");
    expect(byKey["model:generate"].detail).toContain("not installed");
    expect(byKey.exa.state).toBe("attention");
    expect(byKey.github.state).toBe("optional");
  });

  it("says an accepted unencrypted volume is ready by choice", () => {
    const items = checklist(
      aStatus([], { encryption: "unencrypted", unencryptedAccepted: true, warning: "stored without disk encryption, by your choice" }) as never,
      { exa: true },
    );
    const enc = items.find((i) => i.key === "encryption")!;
    expect(enc.state).toBe("ready");
    expect(enc.detail).toContain("by your choice");
  });

  it("treats no backend answer as everything needing attention", () => {
    const items = checklist(null, {});
    expect(items.filter((i) => i.state === "ready")).toHaveLength(0);
  });
});

describe("SystemCheck", () => {
  it("summarises what needs attention and links each item to its section", async () => {
    state.status = aStatus(["ollama"]);
    state.credentials = { exa: false, cloud: false, github: false };
    render(() => (
      <>
        <SystemCheck />
        <section id="provider-keys" aria-label="Provider keys" />
        <section id="setup" aria-label="Setup" />
      </>
    ));
    const summary = await screen.findByLabelText("System check summary");
    // Ollama, the generation model, and the Exa key.
    await waitFor(() => expect(summary).toHaveTextContent("3 items need attention"));

    const exa = screen.getByLabelText("Check Exa key");
    expect(exa).toHaveAttribute("data-state", "attention");
    expect(exa).toHaveTextContent("no key stored");
    // Ready rows offer no link; the GitHub row is optional, not a problem.
    expect(screen.queryByLabelText(/Go to .* for Embedding model/)).toBeNull();
    expect(screen.getByLabelText("Check GitHub token")).toHaveAttribute("data-state", "optional");

    const target = screen.getByLabelText("Provider keys");
    const scrolled = vi.fn();
    (target as HTMLElement).scrollIntoView = scrolled;
    fireEvent.click(screen.getByLabelText("Go to provider keys for Exa key"));
    expect(scrolled).toHaveBeenCalled();
    expect(document.activeElement).toBe(target);
  });

  it("says so when everything is set up, and rechecks on demand", async () => {
    state.status = aStatus([], {
      models: [
        { role: "embed", model: "a", approxBytes: 1, installed: true, state: "ready" },
        { role: "classify", model: "b", approxBytes: 1, installed: true, state: "ready" },
        { role: "generate", model: "c", approxBytes: 1, installed: true, state: "ready" },
      ],
    });
    render(() => <SystemCheck />);
    expect(await screen.findByText("Everything this application needs is set up.")).toBeTruthy();
    fireEvent.click(screen.getByLabelText("Check the system again"));
    await waitFor(() => expect(setupMocks.State).toHaveBeenCalledTimes(2));
  });
});
