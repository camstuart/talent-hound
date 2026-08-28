import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@solidjs/testing-library";
import FirstRunWizard from "./FirstRunWizard";

// The Go backend is not running: bindings are mocked. Every fixture is invented.
const { state, setupMocks, modelMocks } = vi.hoisted(() => {
  const state = { status: null as Record<string, unknown> | null, chooseError: "" };
  return {
    state,
    modelMocks: {
      Options: vi.fn(async () => ({
        models: [
          { role: "embed", model: "nomic-embed-text", purpose: "Turns documents into evidence", power: "recommended", approxBytes: 274 * 1024 ** 2, installed: true, pulling: false },
          { role: "generate", model: "qwen2.5:7b-instruct", purpose: "Writes things", power: "recommended", approxBytes: 4.7 * 1024 ** 3, installed: false, pulling: false },
          { role: "generate", model: "qwen3:8b", purpose: "Writes things", power: "most capable", approxBytes: 5.2 * 1024 ** 3, installed: true, pulling: false },
        ],
        freeDiskBytes: 40 * 1024 ** 3,
      })),
      Assign: vi.fn(async (input: Record<string, unknown>) => ({ id: 1, revision: 1, ...input })),
    },
    setupMocks: {
      State: vi.fn(async () => state.status),
      Acknowledgements: vi.fn(async () => ["I have the authority to hold this data."]),
      ChooseFolder: vi.fn(async () => {
        if (state.chooseError) throw new Error(state.chooseError);
      }),
      SetScope: vi.fn(async () => undefined),
      AcceptUnencrypted: vi.fn(async () => undefined),
      Recheck: vi.fn(async () => "unencrypted"),
      Acknowledge: vi.fn(async () => undefined),
      PullModel: vi.fn(async () => ({ id: 1 })),
      DeclineModel: vi.fn(async () => undefined),
    },
  };
});

vi.mock("../../bindings/camstuart/talent-hound", () => ({ SetupService: setupMocks, ModelService: modelMocks }));

const ORDER = [
  "data_folder",
  "encryption",
  "sidecar",
  "ollama",
  "models",
  "acknowledgement",
  "first_initiative",
];

const aStatus = (next: string, over: Record<string, unknown> = {}) => ({
  next,
  complete: next === "complete",
  steps: ORDER.map((step) => ({
    step,
    satisfied: ORDER.indexOf(step) < ORDER.indexOf(next),
    detail: step === next ? `${step} is not done` : "",
  })),
  models: [
    { role: "embed", model: "nomic-embed-text", approxBytes: 274 * 1024 * 1024, installed: true, state: "ready" },
    { role: "generate", model: "qwen2.5:7b-instruct", approxBytes: 4700 * 1024 * 1024, installed: false, state: "model_missing" },
  ],
  dataFolder: "C:\\Talent Hound",
  scope: "real",
  encryption: "encrypted",
  realData: true,
  realDataWhy: "",
  unencryptedAccepted: false,
  warning: "",
  version: "0.1.0-poc",
  acknowledged: false,
  ...over,
});

const clickWhenReady = async (label: string) => {
  const button = await screen.findByLabelText(label);
  await waitFor(() => expect((button as HTMLButtonElement).disabled).toBe(false));
  fireEvent.click(button);
};

beforeEach(() => {
  state.status = aStatus("models");
  state.chooseError = "";
  vi.clearAllMocks();
});

describe("FirstRunWizard", () => {
  it("shows every step in order, with the one to be on", async () => {
    render(() => <FirstRunWizard />);
    const steps = await screen.findByLabelText("Setup steps");
    const text = steps.textContent ?? "";
    // Order is the PRD's order.
    expect(text.indexOf("data folder")).toBeGreaterThanOrEqual(0);
    expect(text.indexOf("Choose the data folder")).toBeLessThan(text.indexOf("Verify Ollama"));
    expect(text.indexOf("Verify Ollama")).toBeLessThan(text.indexOf("Create the first initiative"));
    // Earlier steps are done, later ones are simply not reached.
    expect(text).toContain("not reached");
    expect(await screen.findByLabelText("Setup position")).toHaveTextContent("Install the required models");
  });

  it("names the missing dependency rather than reporting a generic failure", async () => {
    state.status = aStatus("ollama");
    state.status.steps = ORDER.map((step) => ({
      step,
      satisfied: ORDER.indexOf(step) < ORDER.indexOf("ollama"),
      detail: step === "ollama" ? "Ollama is not reachable at http://localhost:11434" : "",
    }));
    render(() => <FirstRunWizard />);
    const why = await screen.findByLabelText("Why ollama is not done");
    // And how to fix it, not only that it is broken.
    const how = await screen.findByLabelText("How to set up Ollama");
    expect(how).toHaveTextContent("ollama.com/download");
    await clickWhenReady("Check Ollama again");
    await waitFor(() => expect(setupMocks.State.mock.calls.length).toBeGreaterThan(1));
    expect(why.textContent).toContain("Ollama is not reachable");
    expect(why.textContent).toContain("11434");
  });

  it("lists required models with their download sizes and offers both answers", async () => {
    render(() => <FirstRunWizard />);
    const models = await screen.findByLabelText("Required models");
    expect(models.textContent).toContain("nomic-embed-text");
    expect(models.textContent).toContain("4.6 GB");
    expect(models.textContent).toContain("installed");

    // A declined pull is an answer, not a failure.
    await clickWhenReady("Skip the generate model for now");
    await waitFor(() => expect(setupMocks.DeclineModel).toHaveBeenCalledWith("generate"));
    await clickWhenReady("Download the generate model");
    await waitFor(() => expect(setupMocks.PullModel).toHaveBeenCalledWith("generate"));
  });

  it("assigns an installed model the moment it is chosen, showing free disk space", async () => {
    render(() => <FirstRunWizard />);
    await screen.findByText(/40\.0 GB free on this disk/);
    const select = (await screen.findByLabelText("Model for generate")) as HTMLSelectElement;
    await waitFor(() => expect(select.options.length).toBeGreaterThan(1));
    // The persisted assignment shows even though it is not installed yet.
    expect(select.value).toBe("qwen2.5:7b-instruct");
    fireEvent.change(select, { target: { value: "qwen3:8b" } });
    await waitFor(() =>
      expect(modelMocks.Assign).toHaveBeenCalledWith({ role: "generate", endpoint: "", model: "qwen3:8b", digest: "", params: "" }),
    );
  });

  it("says why real data is blocked, and does not switch scope on its own", async () => {
    state.status = aStatus("encryption", {
      realData: false,
      realDataWhy: "the volume holding the data folder is not encrypted",
      encryption: "unencrypted",
    });
    render(() => <FirstRunWizard />);
    expect(await screen.findByLabelText("Why real data is blocked")).toHaveTextContent("not encrypted");
    // Still real scope: nothing switched it.
    expect(await screen.findByLabelText("Data scope")).toHaveTextContent("real");
    expect(setupMocks.SetScope).not.toHaveBeenCalled();
  });

  it("offers to store without disk encryption, as a recorded choice", async () => {
    state.status = aStatus("encryption", {
      realData: false,
      realDataWhy: "the encryption state of the volume could not be checked, so it is not treated as encrypted",
      encryption: "unavailable",
    });
    render(() => <FirstRunWizard />);
    await clickWhenReady("Store candidate data without disk encryption");
    await waitFor(() => expect(setupMocks.AcceptUnencrypted).toHaveBeenCalledWith(true));
    expect(setupMocks.SetScope).not.toHaveBeenCalled();
  });

  it("says the choice back while it is in force, and lets it be withdrawn", async () => {
    state.status = aStatus("sidecar", {
      encryption: "unencrypted",
      unencryptedAccepted: true,
      warning: "candidate data is stored without disk encryption, by your choice",
    });
    render(() => <FirstRunWizard />);
    expect(await screen.findByLabelText("Unencrypted storage warning")).toHaveTextContent("by your choice");
    expect(screen.queryByLabelText("Store candidate data without disk encryption")).toBeNull();
    expect(screen.queryByLabelText("Why real data is blocked")).toBeNull();
    await clickWhenReady("Require disk encryption again");
    await waitFor(() => expect(setupMocks.AcceptUnencrypted).toHaveBeenCalledWith(false));
  });

  it("distinguishes a volume that could not be checked from one that is not encrypted", async () => {
    state.status = aStatus("encryption", {
      realData: false,
      realDataWhy: "the encryption state of the volume could not be checked, so it is not treated as encrypted",
      encryption: "unavailable",
    });
    render(() => <FirstRunWizard />);
    expect(await screen.findByLabelText("Why real data is blocked")).toHaveTextContent("could not be checked");
  });

  it("records the acknowledgement and shows what was acknowledged", async () => {
    state.status = aStatus("acknowledgement");
    render(() => <FirstRunWizard />);
    const terms = await screen.findByLabelText("Data handling");
    expect(terms.textContent).toContain("authority to hold");
    await clickWhenReady("Acknowledge these responsibilities");
    await waitFor(() => expect(setupMocks.Acknowledge).toHaveBeenCalled());
  });

  it("shows the backend's own words when a folder is refused", async () => {
    state.status = aStatus("data_folder");
    state.chooseError = "the folder cannot be written to: permission denied";
    render(() => <FirstRunWizard />);
    fireEvent.input(await screen.findByLabelText("Data folder"), { target: { value: "/read-only" } });
    await clickWhenReady("Use this data folder");
    await screen.findByText(/cannot be written to/);
  });

  it("shows the application version", async () => {
    render(() => <FirstRunWizard />);
    expect(await screen.findByLabelText("Application version")).toHaveTextContent("0.1.0-poc");
  });
});
