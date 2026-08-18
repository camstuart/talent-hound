import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@solidjs/testing-library";
import DiagnosticsPanel from "./DiagnosticsPanel";

// The Go backend is not running: bindings are mocked. Every fixture is invented.
const { state, mocks } = vi.hoisted(() => {
  const state = { deleteError: "" };
  return {
    state,
    mocks: {
      DiagnosticsService: {
        Diagnostics: vi.fn(async () => ({
          version: "0.1.0-poc",
          schemaVersion: 16,
          buildSchema: 16,
          platform: "windows/amd64",
          dataFolder: "C:\\Talent Hound",
          logsFolder: "C:\\Talent Hound\\logs",
          encryption: "encrypted",
          scope: "real",
          realData: true,
          sidecar: "available",
          ollama: "unavailable",
          models: [{ kind: "embed: ready", count: 1 }],
          counts: [
            { kind: "candidates", count: 3 },
            { kind: "artifacts", count: 5 },
          ],
          jobs: [{ kind: "failed: sidecar_missing", count: 2 }],
        })),
        RecoveryProcedure: vi.fn(async () => ({
          dataFolder: "C:\\Talent Hound",
          steps: [
            "Close Talent Hound completely, then copy C:\\Talent Hound somewhere safe.",
            "Re-enter provider credentials: they live in the Windows credential store, not in the folder.",
          ],
        })),
        LogsFolder: vi.fn(async () => "C:\\Talent Hound\\logs"),
        OpenLogsFolder: vi.fn(async () => "C:\\Talent Hound\\logs"),
        DeleteAll: vi.fn(async (confirmation: string) => {
          if (state.deleteError) throw new Error(state.deleteError);
          return confirmation;
        }),
      },
    },
  };
});

vi.mock("../../bindings/camstuart/talent-hound", () => mocks);

const clickWhenReady = async (label: string) => {
  const button = await screen.findByLabelText(label);
  await waitFor(() => expect((button as HTMLButtonElement).disabled).toBe(false));
  fireEvent.click(button);
};

beforeEach(() => {
  state.deleteError = "";
  vi.clearAllMocks();
});

describe("DiagnosticsPanel", () => {
  it("reports versions, folders, availability, and counts", async () => {
    render(() => <DiagnosticsPanel />);
    expect(await screen.findByLabelText("Application version")).toHaveTextContent("0.1.0-poc");
    expect(await screen.findByLabelText("Schema version")).toHaveTextContent("v16");
    expect(await screen.findByLabelText("Data folder")).toHaveTextContent("C:\\Talent Hound");
    expect(await screen.findByLabelText("Dependency availability")).toHaveTextContent("Ollama: unavailable");
    expect(await screen.findByLabelText("Record counts")).toHaveTextContent("candidates: 3");
    expect(await screen.findByLabelText("Job outcomes")).toHaveTextContent("failed: sidecar_missing");
  });

  it("reports the logs folder path", async () => {
    render(() => <DiagnosticsPanel />);
    await clickWhenReady("Open the logs folder");
    await waitFor(() => expect(mocks.DiagnosticsService.OpenLogsFolder).toHaveBeenCalled());
    expect(await screen.findByLabelText("Logs folder")).toHaveTextContent("logs");
  });

  it("names the resolved folder in the recovery procedure", async () => {
    render(() => <DiagnosticsPanel />);
    const recovery = await screen.findByLabelText("Recovery procedure");
    expect(recovery.textContent).toContain("C:\\Talent Hound");
    expect(recovery.textContent).toContain("credential");
  });

  // Confirming "yes" to a folder described in words is how the wrong folder
  // gets deleted.
  it("requires the exact folder before deleting anything", async () => {
    state.deleteError = "nothing was deleted: to delete everything, confirm the exact folder C:\\Talent Hound";
    render(() => <DiagnosticsPanel />);
    fireEvent.input(await screen.findByLabelText("Folder to confirm"), { target: { value: "yes" } });
    await clickWhenReady("Delete everything in the data folder");
    await screen.findByText(/nothing was deleted/);
    expect(screen.queryByLabelText("Deletion outcome")).toBeNull();
  });

  it("deletes when the exact folder is confirmed", async () => {
    render(() => <DiagnosticsPanel />);
    fireEvent.input(await screen.findByLabelText("Folder to confirm"), {
      target: { value: "C:\\Talent Hound" },
    });
    await clickWhenReady("Delete everything in the data folder");
    await waitFor(() =>
      expect(mocks.DiagnosticsService.DeleteAll).toHaveBeenCalledWith("C:\\Talent Hound"),
    );
    expect(await screen.findByLabelText("Deletion outcome")).toHaveTextContent("C:\\Talent Hound");
  });

  it("offers nothing that enables telemetry", async () => {
    const { container } = render(() => <DiagnosticsPanel />);
    await screen.findByLabelText("Application version");
    const text = (container.textContent ?? "").toLowerCase();
    for (const word of ["telemetry", "analytics", "usage data", "send report"]) {
      expect(text).not.toContain(word);
    }
  });
});
