import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@solidjs/testing-library";
import App from "./App";

// The Go backend is not running: every binding call is mocked. Fixtures are
// invented — no real candidate information lives in this repository.
const { initiatives, mocks } = vi.hoisted(() => {
  const initiatives: Record<string, unknown>[] = [];
  const find = (id: number) => initiatives.find((i) => i.id === id);
  // IDs restart with each test, because beforeEach empties the array.
  const nextId = () => initiatives.reduce((max, i) => Math.max(max, Number(i.id)), 0) + 1;
  // Replace rather than mutate, as the real backend does: Solid's <For> keys on
  // object identity, so an in-place edit would never reach the DOM.
  const update = (id: number, patch: Record<string, unknown>) => {
    const at = initiatives.findIndex((i) => i.id === id);
    initiatives[at] = { ...initiatives[at], ...patch };
    return initiatives[at];
  };
  return {
    initiatives,
    mocks: {
      Create: vi.fn(async (name: string, type: string, candidateIDs: number[]) => {
        const created = {
          id: nextId(),
          name,
          type,
          status: "active",
          candidateId: candidateIDs[0] ?? null,
          createdAt: new Date().toISOString(),
          updatedAt: new Date().toISOString(),
        };
        initiatives.push(created);
        return created;
      }),
      List: vi.fn(async (includeArchived: boolean) =>
        initiatives.filter((i) => includeArchived || i.status === "active"),
      ),
      Get: vi.fn(async (id: number) => find(id) ?? Promise.reject(new Error(`loading initiative ${id}`))),
      Rename: vi.fn(async (id: number, name: string) => update(id, { name })),
      Archive: vi.fn(async (id: number) => {
        if (find(id)!.status === "archived") throw new Error(`initiative ${id} is already archived`);
        return update(id, { status: "archived" });
      }),
      Reopen: vi.fn(async (id: number) => update(id, { status: "active" })),
      ListCandidates: vi.fn(async () => []),
      ListCompanies: vi.fn(async () => []),
      ListRoles: vi.fn(async () => []),
      CreateCandidate: vi.fn(async (c: { fullName: string }) => ({ id: 42, ...c })),
      ListForTarget: vi.fn(async () => []),
      ListOrphans: vi.fn(async () => []),
    },
  };
});

vi.mock("../bindings/camstuart/talent-hound", () => ({
  InitiativeService: {
    Create: mocks.Create,
    List: mocks.List,
    Get: mocks.Get,
    Rename: mocks.Rename,
    Archive: mocks.Archive,
    Reopen: mocks.Reopen,
  },
  RecordService: {
    ListCandidates: mocks.ListCandidates,
    ListCompanies: mocks.ListCompanies,
    ListRoles: mocks.ListRoles,
    CreateCandidate: mocks.CreateCandidate,
  },
  ArtifactService: {
    ListForTarget: mocks.ListForTarget,
    ListOrphans: mocks.ListOrphans,
  },
  // The shell mounts the status strip and asks whether setup is finished.
  SetupService: {
    State: vi.fn(async () => ({ next: "complete", scope: "real", realData: true })),
    Scope: vi.fn(async () => ({ scope: "real", realData: true })),
  },
  ModelService: { Check: vi.fn(async () => []) },
  CloudService: { Tasks: vi.fn(async () => []) },
}));

beforeEach(() => {
  initiatives.length = 0;
  Object.values(mocks).forEach((m) => m.mockClear());
});

async function createInitiative(name: string, type: string, candidateName?: string) {
  fireEvent.click(screen.getByLabelText("New initiative"));
  const dialog = await screen.findByRole("dialog", { name: "New initiative" });
  fireEvent.input(screen.getByPlaceholderText("Initiative name"), { target: { value: name } });
  fireEvent.change(screen.getByLabelText("Initiative type"), { target: { value: type } });
  if (candidateName !== undefined) {
    fireEvent.input(screen.getByPlaceholderText("Candidate full name"), { target: { value: candidateName } });
  }
  fireEvent.click(screen.getByText("Create"));
  await waitFor(() => expect(dialog).not.toBeInTheDocument());
}

describe("App", () => {
  it("loads existing initiatives into the sidebar on mount", async () => {
    initiatives.push({ id: 7, name: "Existing hunt", type: "talent_search", status: "active" });
    render(() => <App />);

    await waitFor(() => {
      expect(screen.getByText("Existing hunt")).toBeInTheDocument();
    });
    expect(mocks.List).toHaveBeenCalledWith(false);
    // Listed but not opened: no initiative tab yet.
    expect(screen.queryByRole("tab", { name: /Existing hunt/ })).not.toBeInTheDocument();
  });

  it("creates an initiative via the modal, adds it to the sidebar and opens a tab", async () => {
    render(() => <App />);

    await createInitiative("Hire Go devs", "talent_search");

    expect(mocks.Create).toHaveBeenCalledWith("Hire Go devs", "talent_search", []);
    expect(screen.getByRole("tab", { name: /Hire Go devs/ })).toBeInTheDocument();
    expect(document.querySelectorAll('[data-icon="talent_search"]').length).toBeGreaterThanOrEqual(2);
  });

  it("creates the one candidate a job search initiative needs", async () => {
    render(() => <App />);

    await createInitiative("Find a Go role", "job_search", "Priya Raman");

    expect(mocks.CreateCandidate).toHaveBeenCalledWith({ fullName: "Priya Raman" });
    expect(mocks.Create).toHaveBeenCalledWith("Find a Go role", "job_search", [42]);
  });

  it("closes a tab with its close button, keeping the initiative in the sidebar", async () => {
    render(() => <App />);

    await createInitiative("BD pipeline", "business_development");
    fireEvent.click(screen.getByLabelText("Close BD pipeline"));

    expect(screen.queryByRole("tab", { name: /BD pipeline/ })).not.toBeInTheDocument();
    expect(screen.getByText("BD pipeline")).toBeInTheDocument();
    expect(screen.getByText(/Create an initiative from the panel/)).toBeInTheDocument();
  });

  it("activates the neighbouring tab when the active tab is closed", async () => {
    render(() => <App />);

    await createInitiative("First", "job_search", "Priya Raman");
    await createInitiative("Second", "talent_search");

    expect(screen.getByRole("tab", { name: /Second/ })).toHaveAttribute("aria-selected", "true");

    fireEvent.click(screen.getByLabelText("Close Second"));

    expect(screen.getByRole("tab", { name: /First/ })).toHaveAttribute("aria-selected", "true");
  });

  it("shows the four workspace areas for every initiative type", async () => {
    render(() => <App />);
    await createInitiative("Hire Go devs", "talent_search");

    const areas = screen.getByRole("tablist", { name: "Initiative areas" });
    for (const label of ["Context", "Research", "Matches", "Drafts"]) {
      expect(areas).toHaveTextContent(label);
    }
    // Context is open first and says what it will hold.
    expect(screen.getByRole("tabpanel", { name: "Context" })).toBeInTheDocument();

    fireEvent.click(screen.getByRole("tab", { name: "Drafts" }));
    expect(screen.getByRole("tabpanel", { name: "Drafts" })).toHaveTextContent(/Outreach drafts/);
    // Still the same initiative: switching areas is not switching tabs.
    expect(screen.getByRole("tab", { name: /Hire Go devs/ })).toHaveAttribute("aria-selected", "true");
  });

  it("says that talent search and business development are shells", async () => {
    render(() => <App />);

    await createInitiative("Hire Go devs", "talent_search");
    expect(screen.getByText(/Talent Search is a workspace shell/)).toBeInTheDocument();

    await createInitiative("BD pipeline", "business_development");
    expect(screen.getByText(/Business Development is a workspace shell/)).toBeInTheDocument();
  });

  it("does not call a job search initiative a shell", async () => {
    render(() => <App />);

    await createInitiative("Find a Go role", "job_search", "Priya Raman");

    expect(screen.queryByText(/workspace shell/)).not.toBeInTheDocument();
  });

  it("renames an initiative in place", async () => {
    render(() => <App />);
    await createInitiative("Original", "talent_search");

    fireEvent.click(screen.getByText("Rename"));
    const input = screen.getByLabelText("New name") as HTMLInputElement;
    fireEvent.input(input, { target: { value: "Renamed" } });
    fireEvent.keyDown(input, { key: "Enter" });

    await waitFor(() => expect(mocks.Rename).toHaveBeenCalledWith(1, "Renamed"));
    await waitFor(() => expect(screen.getByRole("tab", { name: /Renamed/ })).toBeInTheDocument());
  });

  it("archives and reopens, labelling the state", async () => {
    render(() => <App />);
    await createInitiative("Hire Go devs", "talent_search");

    expect(screen.getByText("Active")).toBeInTheDocument();
    expect(screen.queryByText("Reopen")).not.toBeInTheDocument();

    fireEvent.click(screen.getByText("Archive"));

    // The workspace stays open and is labelled, and only Reopen is offered.
    await waitFor(() => expect(screen.getAllByText("Archived").length).toBeGreaterThan(0));
    expect(screen.getByText("Reopen")).toBeInTheDocument();
    expect(screen.queryByText("Archive")).not.toBeInTheDocument();

    fireEvent.click(screen.getByText("Reopen"));
    await waitFor(() => expect(screen.getByText("Archive")).toBeInTheDocument());
    expect(screen.getByText("Active")).toBeInTheDocument();
  });

  it("shows the backend's own words when a lifecycle action is refused", async () => {
    render(() => <App />);
    await createInitiative("Hire Go devs", "talent_search");

    initiatives[0].status = "archived";
    fireEvent.click(screen.getByText("Archive"));

    await waitFor(() => expect(screen.getByText("initiative 1 is already archived")).toBeInTheDocument());
  });

  it("asks the backend for archived initiatives only when the filter is on", async () => {
    render(() => <App />);
    await waitFor(() => expect(mocks.List).toHaveBeenCalledWith(false));

    fireEvent.click(screen.getByLabelText("Show archived"));

    await waitFor(() => expect(mocks.List).toHaveBeenCalledWith(true));
  });
});
