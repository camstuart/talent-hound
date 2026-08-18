import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@solidjs/testing-library";
import DeletePanel from "./DeletePanel";

// The Go backend is not running: bindings are mocked. Fixtures are invented.
const { state, deletionMocks, recordMocks } = vi.hoisted(() => {
  const state = {
    preview: null as Record<string, unknown> | null,
    deleteError: "",
  };
  return {
    state,
    recordMocks: {
      ListCandidates: vi.fn(async () => [{ id: 1, fullName: "Kalinda Reyes" }]),
      ListRoles: vi.fn(async () => [{ id: 2, title: "Platform engineer" }]),
    },
    deletionMocks: {
      PreviewCandidate: vi.fn(async () => state.preview),
      PreviewRolePurge: vi.fn(async () => state.preview),
      DeleteCandidate: vi.fn(async () => {
        if (state.deleteError) throw new Error(state.deleteError);
        return { blocked: false, blockers: [], removes: [] };
      }),
      PurgeRole: vi.fn(async () => ({ blocked: false, blockers: [], removes: [] })),
    },
  };
});

vi.mock("../../bindings/camstuart/talent-hound", () => ({
  DeletionService: deletionMocks,
  RecordService: recordMocks,
}));

const aPreview = (over: Record<string, unknown> = {}) => ({
  blocked: false,
  blockers: [],
  choice: "",
  removes: [
    { kind: "profile versions", count: 2, detail: [] },
    { kind: "candidate-only artifacts", count: 1, detail: [] },
  ],
  ...over,
});

const clickWhenReady = async (label: string) => {
  const button = await screen.findByLabelText(label);
  await waitFor(() => expect((button as HTMLButtonElement).disabled).toBe(false));
  fireEvent.click(button);
};

beforeEach(() => {
  state.preview = aPreview();
  state.deleteError = "";
  vi.clearAllMocks();
});

describe("DeletePanel", () => {
  it("lists exactly what would be removed rather than saying related data", async () => {
    render(() => <DeletePanel />);
    await clickWhenReady("Preview deleting Kalinda Reyes");

    const removes = await screen.findByLabelText("What would be removed");
    expect(removes.textContent).toContain("profile versions: 2");
    expect(removes.textContent).toContain("candidate-only artifacts: 1");
    // Nothing has been deleted by previewing.
    expect(deletionMocks.DeleteCandidate).not.toHaveBeenCalled();
  });

  // A refusal that says "cannot delete" and stops is one the recruiter cannot
  // act on.
  it("names what is blocking a deletion", async () => {
    state.preview = aPreview({
      blocked: true,
      blockers: ['the archived initiative "Find a Go role" references this candidate'],
    });
    render(() => <DeletePanel />);
    await clickWhenReady("Preview deleting Kalinda Reyes");

    const blockers = await screen.findByLabelText("What is blocking this deletion");
    expect(blockers.textContent).toContain("archived initiative");
    expect(blockers.textContent).toContain("Find a Go role");
    // And there is nothing to confirm.
    expect(screen.queryByLabelText("Confirm this deletion")).toBeNull();
  });

  // Neither default is safe, so the application refuses to choose.
  it("offers both branches for a shared artifact and neither by default", async () => {
    state.preview = aPreview({
      blocked: true,
      choice:
        "these artifacts are attached to other records too, and may contain candidate information: " +
        "delete them everywhere, or keep them under their other links",
      removes: [{ kind: "artifacts attached elsewhere", count: 1, detail: ["Reyes resume"] }],
    });
    render(() => <DeletePanel />);
    await clickWhenReady("Preview deleting Kalinda Reyes");

    const choice = await screen.findByLabelText("Choice required");
    expect(choice.textContent).toContain("may contain candidate information");
    // Both branches offered, and no plain confirm.
    expect(await screen.findByLabelText("Delete the shared artifacts everywhere")).toBeTruthy();
    expect(await screen.findByLabelText("Keep the shared artifacts under their other links")).toBeTruthy();
    expect(screen.queryByLabelText("Confirm this deletion")).toBeNull();

    await clickWhenReady("Keep the shared artifacts under their other links");
    await waitFor(() =>
      expect(deletionMocks.DeleteCandidate).toHaveBeenCalledWith(1, "retain_under_other_links"),
    );
  });

  it("confirms an unblocked deletion", async () => {
    render(() => <DeletePanel />);
    await clickWhenReady("Preview deleting Kalinda Reyes");
    await clickWhenReady("Confirm this deletion");
    await waitFor(() => expect(deletionMocks.DeleteCandidate).toHaveBeenCalledWith(1, ""));
    await screen.findByLabelText("Deletion outcome");
  });

  it("cancels without deleting", async () => {
    render(() => <DeletePanel />);
    await clickWhenReady("Preview deleting Kalinda Reyes");
    await clickWhenReady("Cancel this deletion");

    await waitFor(() => expect(screen.queryByLabelText("What would be removed")).toBeNull());
    expect(deletionMocks.DeleteCandidate).not.toHaveBeenCalled();
  });

  it("previews a role purge with its referencing initiatives", async () => {
    state.preview = aPreview({
      removes: [
        { kind: "referencing initiatives", count: 1, detail: ["Find a Go role"] },
        { kind: "source listings, current and historical", count: 2, detail: [] },
      ],
    });
    render(() => <DeletePanel />);
    await clickWhenReady("Preview purging Platform engineer");

    const removes = await screen.findByLabelText("What would be removed");
    expect(removes.textContent).toContain("referencing initiatives");
    expect(removes.textContent).toContain("Find a Go role");
    expect(removes.textContent).toContain("source listings, current and historical: 2");
  });

  it("shows the backend's own words when a deletion is refused", async () => {
    state.deleteError = "this candidate cannot be deleted yet: an initiative references them";
    render(() => <DeletePanel />);
    await clickWhenReady("Preview deleting Kalinda Reyes");
    await clickWhenReady("Confirm this deletion");
    await screen.findByText(/cannot be deleted yet/);
  });
});
