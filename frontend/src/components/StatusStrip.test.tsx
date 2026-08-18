import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@solidjs/testing-library";
import StatusStrip from "./StatusStrip";

// The Go backend is not running: bindings are mocked. Every fixture is invented.
const { state, mocks } = vi.hoisted(() => {
  const state = {
    scope: "real",
    realData: true,
    models: [] as Record<string, unknown>[],
    tasks: [] as Record<string, unknown>[],
    modelsThrow: false,
  };
  return {
    state,
    mocks: {
      SetupService: {
        Scope: vi.fn(async () => ({ scope: state.scope, realData: state.realData })),
      },
      ModelService: {
        Check: vi.fn(async () => {
          if (state.modelsThrow) throw new Error("Ollama is not running");
          return state.models;
        }),
      },
      CloudService: { Tasks: vi.fn(async () => state.tasks) },
    },
  };
});

vi.mock("../../bindings/camstuart/talent-hound", () => mocks);

const ready = [
  { role: "embed", model: "nomic-embed-text", state: "ready" },
  { role: "generate", model: "qwen2.5:7b-instruct", state: "ready" },
];

beforeEach(() => {
  state.scope = "real";
  state.realData = true;
  state.models = ready;
  state.tasks = [];
  state.modelsThrow = false;
  vi.clearAllMocks();
});

describe("StatusStrip", () => {
  it("shows the initiative, the scope, the models, the override, and connectivity", async () => {
    render(() => <StatusStrip initiativeId={7} initiativeName="Find a Go role" />);

    expect(await screen.findByLabelText("Active initiative")).toHaveTextContent("Find a Go role");
    expect(await screen.findByLabelText("Data scope")).toHaveTextContent("Real scope");
    await waitFor(async () =>
      expect(await screen.findByLabelText("Selected models")).toHaveTextContent("nomic-embed-text"),
    );
    expect(await screen.findByLabelText("Cloud override")).toHaveTextContent("Local only");
    await waitFor(async () =>
      expect(await screen.findByLabelText("Connectivity")).toHaveTextContent("Local models reachable"),
    );
  });

  it("says demo scope distinctly, and says candidate data is blocked", async () => {
    state.scope = "demo";
    state.realData = false;
    render(() => <StatusStrip />);
    const scope = await screen.findByLabelText("Data scope");
    await waitFor(() => expect(scope).toHaveTextContent("Demo scope"));
    expect(scope).toHaveTextContent("candidate data blocked");
  });

  it("reflects a provider failure without losing the rest of the state", async () => {
    state.modelsThrow = true;
    render(() => <StatusStrip initiativeId={7} initiativeName="Find a Go role" />);

    await waitFor(async () =>
      expect(await screen.findByLabelText("Connectivity")).toHaveTextContent("Offline"),
    );
    // Everything else still reads correctly: this is exactly when it matters.
    expect(await screen.findByLabelText("Active initiative")).toHaveTextContent("Find a Go role");
    expect(await screen.findByLabelText("Data scope")).toHaveTextContent("Real scope");
  });

  it("shows a cloud override while it is in force", async () => {
    state.tasks = [
      { task: "drafting", approved: true, denied: false, reason: "" },
      { task: "chat", approved: false, denied: false, reason: "" },
    ];
    render(() => <StatusStrip initiativeId={7} initiativeName="Find a Go role" />);
    const override = await screen.findByLabelText("Cloud override");
    await waitFor(() => expect(override).toHaveTextContent("Cloud override in force"));
    expect(override).toHaveTextContent("drafting");
    expect(override).not.toHaveTextContent("chat");
  });

  it("says so plainly when no initiative is open", async () => {
    render(() => <StatusStrip />);
    expect(await screen.findByLabelText("Active initiative")).toHaveTextContent("No initiative open");
  });
});
