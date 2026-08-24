import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@solidjs/testing-library";
import SettingsPanel from "./SettingsPanel";

// The Go backend is not running: bindings are mocked. No real provider key
// appears anywhere in this repository; the value below is invented.
const inventedKey = "not-a-real-key-VITEST-71c0";

const { state, modelMocks, credentialMocks } = vi.hoisted(() => {
  const state = {
    registry: [] as Record<string, unknown>[],
    statuses: [] as Record<string, unknown>[],
    credentials: {} as Record<string, boolean>,
    assignError: "",
    options: { models: [] as Record<string, unknown>[], freeDiskBytes: 0 } as Record<string, unknown>,
  };
  return {
    state,
    modelMocks: {
      Registry: vi.fn(async () => state.registry),
      Check: vi.fn(async () => state.statuses),
      Assign: vi.fn(async (input: Record<string, unknown>) => {
        if (state.assignError) throw new Error(state.assignError);
        return { id: 1, revision: 1, validation: "unvalidated", ...input };
      }),
      Pull: vi.fn(async () => ({ id: 4, kind: "pull", state: "queued" })),
      Decline: vi.fn(async () => undefined),
      Options: vi.fn(async () => state.options),
    },
    credentialMocks: {
      List: vi.fn(async () => state.credentials),
      Store: vi.fn(async () => undefined),
      Delete: vi.fn(async () => undefined),
      Has: vi.fn(async () => false),
    },
  };
});

vi.mock("../../bindings/camstuart/talent-hound", () => ({
  ModelService: modelMocks,
  CredentialService: credentialMocks,
}));

const resolution = (role: string, over: Record<string, unknown> = {}) => ({
  role,
  inherited: false,
  assignment: { id: 1, role, revision: 1, endpoint: "http://localhost:11434", model: `${role}-model`, digest: "", params: "{}", validation: "unvalidated" },
  ...over,
});

beforeEach(() => {
  document.documentElement.dataset.os = "windows";
  state.registry = [
    resolution("embed"),
    // classify with no row of its own: it resolves to the generate model.
    resolution("classify", {
      inherited: true,
      assignment: { id: 3, role: "generate", revision: 1, endpoint: "http://localhost:11434", model: "generate-model", digest: "", params: "{}", validation: "unvalidated" },
    }),
    resolution("generate"),
  ];
  state.statuses = [
    { role: "embed", model: "embed-model", inherited: false, state: "ready" },
    { role: "classify", model: "generate-model", inherited: true, state: "ready" },
    { role: "generate", model: "generate-model", inherited: false, state: "ready" },
  ];
  state.options = {
    models: [
      { role: "embed", model: "nomic-embed-text", purpose: "Turns documents into searchable evidence", power: "recommended", approxBytes: 274 * 1024 ** 2, installed: true },
      { role: "classify", model: "qwen2.5:7b-instruct", purpose: "Reads profiles", power: "recommended", approxBytes: 4.7 * 1024 ** 3, installed: false },
      { role: "generate", model: "qwen2.5:7b-instruct", purpose: "Writes things", power: "recommended", approxBytes: 4.7 * 1024 ** 3, installed: false },
      { role: "generate", model: "qwen3:8b", purpose: "Writes things", power: "most capable", approxBytes: 5.2 * 1024 ** 3, installed: false },
    ],
    freeDiskBytes: 40 * 1024 ** 3,
  };
  state.credentials = { exa: false, cloud: false };
  state.assignError = "";
  vi.clearAllMocks();
});

describe("SettingsPanel", () => {
  it("does not claim Windows storage when no runtime has answered", async () => {
    delete document.documentElement.dataset.os;
    render(() => <SettingsPanel />);
    await screen.findByText("Provider key storage is unavailable on this platform.");
    expect(credentialMocks.List).not.toHaveBeenCalled();
  });

  it("offers macOS Keychain storage on macOS", async () => {
    document.documentElement.dataset.os = "darwin";
    render(() => <SettingsPanel />);
    await screen.findByText(/Keys are held by macOS Keychain/);
    expect(screen.getByLabelText("Save exa")).toBeEnabled();
    await waitFor(() => expect(credentialMocks.List).toHaveBeenCalled());
  });

  it("lists every role with its model, revision, and status", async () => {
    render(() => <SettingsPanel />);
    await screen.findByText(/embed-model/);
    expect(screen.getAllByText(/revision 1, unvalidated/)).toHaveLength(3);
    expect(screen.getAllByText(/ready/)).toHaveLength(3);
  });

  it("says when classify is inherited rather than assigned", async () => {
    render(() => <SettingsPanel />);
    await screen.findByText(/inherited from generate/);
  });

  it("assigns a model chosen from the curated list", async () => {
    render(() => <SettingsPanel />);
    const select = (await screen.findByLabelText("Model for embed")) as HTMLSelectElement;
    await waitFor(() => expect(select.options.length).toBeGreaterThan(2));
    fireEvent.change(select, { target: { value: "nomic-embed-text" } });
    fireEvent.click(screen.getByLabelText("Assign a model to embed"));
    await waitFor(() =>
      expect(modelMocks.Assign).toHaveBeenCalledWith({
        role: "embed",
        endpoint: "",
        model: "nomic-embed-text",
        digest: "",
        params: "",
      }),
    );
  });


  it("shows how much disk space is free", async () => {
    render(() => <SettingsPanel />);
    await screen.findByText(/40\.0 GB free on this disk/);
  });

  it("marks already-downloaded models in the list", async () => {
    render(() => <SettingsPanel />);
    const select = (await screen.findByLabelText("Model for embed")) as HTMLSelectElement;
    await waitFor(() => expect(select.options.length).toBeGreaterThan(2));
    const label = Array.from(select.options).find((o) => o.value === "nomic-embed-text")?.text ?? "";
    expect(label).toContain("installed");
  });

  it("disables every picker while a download is in progress", async () => {
    state.statuses = [
      { role: "embed", model: "embed-model", inherited: false, state: "pulling" },
      { role: "classify", model: "generate-model", inherited: true, state: "ready" },
      { role: "generate", model: "generate-model", inherited: false, state: "ready" },
    ];
    render(() => <SettingsPanel />);
    await screen.findByText(/downloading now/);
    expect(screen.getByLabelText("Model for generate")).toBeDisabled();
    expect(screen.getByLabelText("Assign a model to generate")).toBeDisabled();
  });

  it("offers a download only when the model is missing, and declining is its own action", async () => {
    state.statuses = [
      { role: "embed", model: "embed-model", inherited: false, state: "model_missing" },
      { role: "classify", model: "generate-model", inherited: true, state: "ready" },
      { role: "generate", model: "generate-model", inherited: false, state: "ready" },
    ];
    render(() => <SettingsPanel />);
    await screen.findByText(/not installed/);
    expect(screen.queryByLabelText("Download the generate model")).toBeNull();

    fireEvent.click(screen.getByLabelText("Decline the embed download"));
    await waitFor(() => expect(modelMocks.Decline).toHaveBeenCalledWith("embed"));

    fireEvent.click(screen.getByLabelText("Download the embed model"));
    await waitFor(() => expect(modelMocks.Pull).toHaveBeenCalledWith("embed"));
  });

  it("shows a distinct message for each failure state", async () => {
    state.statuses = [
      { role: "embed", model: "embed-model", inherited: false, state: "endpoint_unavailable" },
      { role: "classify", model: "generate-model", inherited: true, state: "timeout" },
      { role: "generate", model: "generate-model", inherited: false, state: "out_of_memory" },
    ];
    render(() => <SettingsPanel />);
    await screen.findByText(/Ollama is not running/);
    expect(screen.getByText(/no answer in time/)).toBeTruthy();
    expect(screen.getByText(/not enough memory/)).toBeTruthy();
  });

  it("masks a key, stores it, and clears the field", async () => {
    render(() => <SettingsPanel />);
    const field = (await screen.findByLabelText("exa key")) as HTMLInputElement;
    expect(field.type).toBe("password");

    fireEvent.input(field, { target: { value: inventedKey } });
    fireEvent.click(screen.getByLabelText("Save exa"));
    await waitFor(() => expect(credentialMocks.Store).toHaveBeenCalledWith("exa", inventedKey));
    // The value is gone from the page the moment it has been handed over.
    await waitFor(() => expect((screen.getByLabelText("exa key") as HTMLInputElement).value).toBe(""));
  });

  it("shows whether a key is stored and offers to remove it", async () => {
    state.credentials = { exa: true, cloud: false };
    render(() => <SettingsPanel />);
    await screen.findByText(/— key stored/);
    expect(screen.getByText(/— no key stored/)).toBeTruthy();
    expect(screen.queryByLabelText("Remove cloud")).toBeNull();

    fireEvent.click(screen.getByLabelText("Remove exa"));
    await waitFor(() => expect(credentialMocks.Delete).toHaveBeenCalledWith("exa"));
  });

  it("shows the backend's own words when an assignment is refused", async () => {
    state.assignError = "a required model role must use the local endpoint";
    render(() => <SettingsPanel />);
    fireEvent.click(await screen.findByLabelText("Assign a model to embed"));
    await screen.findByText("a required model role must use the local endpoint");
  });
});
