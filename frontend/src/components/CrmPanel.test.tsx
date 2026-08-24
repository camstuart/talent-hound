import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@solidjs/testing-library";
import CrmPanel from "./CrmPanel";

// The Go backend is not running: bindings are mocked. Every fixture is invented.
const { state, recordMocks, searchMocks, interactionMocks, profileMocks, artifactMocks, extractMocks } = vi.hoisted(() => {
  const state = {
    candidates: [
      { id: 1, fullName: "Alice Amber", location: "Sydney", emails: [], phones: [], compensation: {} },
      { id: 2, fullName: "Bob Blue", location: "Melbourne", emails: [], phones: [], compensation: {} },
    ] as Record<string, unknown>[],
    people: [] as Record<string, unknown>[],
    timeline: [] as Record<string, unknown>[],
  };
  return {
    state,
    recordMocks: {
      SearchCandidates: vi.fn(async () => state.candidates),
      SearchCompanies: vi.fn(async () => []),
      SearchContacts: vi.fn(async () => []),
      ListRoles: vi.fn(async () => []),
      ListCompanies: vi.fn(async () => []),
      GetCandidate: vi.fn(async (id: number) => state.candidates.find((c) => c.id === id)),
      GetCompany: vi.fn(async () => null),
      GetContact: vi.fn(async () => null),
      GetRole: vi.fn(async () => null),
      CreateCandidate: vi.fn(async (c: Record<string, unknown>) => ({ id: 9, ...c })),
      UpdateCandidate: vi.fn(async (c: Record<string, unknown>) => c),
      UpdateCompany: vi.fn(async (c: Record<string, unknown>) => c),
      UpdateContact: vi.fn(async (c: Record<string, unknown>) => c),
      UpdateRole: vi.fn(async (c: Record<string, unknown>) => c),
    },
    searchMocks: {
      People: vi.fn(async () => state.people),
    },
    interactionMocks: {
      Timeline: vi.fn(async () => state.timeline),
      Log: vi.fn(async (i: Record<string, unknown>) => ({ id: 5, ...i })),
      Update: vi.fn(async (i: Record<string, unknown>) => i),
      Delete: vi.fn(async () => undefined),
    },
    profileMocks: {
      InUse: vi.fn(async () => null),
    },
    artifactMocks: {
      ListForTarget: vi.fn(async () => []),
      ListOrphans: vi.fn(async () => []),
      Create: vi.fn(async (input: Record<string, unknown>) => ({ id: 1, ...input })),
      Rename: vi.fn(async () => ({ id: 1 })),
      Detach: vi.fn(async () => undefined),
      Link: vi.fn(async () => undefined),
    },
    extractMocks: {
      Extract: vi.fn(async () => ({ id: 5 })),
      Markdown: vi.fn(async () => ""),
    },
  };
});

vi.mock("../../bindings/camstuart/talent-hound", () => ({
  RecordService: recordMocks,
  SearchService: searchMocks,
  InteractionService: interactionMocks,
  CandidateProfileService: profileMocks,
  ArtifactService: artifactMocks,
  ExtractService: extractMocks,
}));

beforeEach(() => {
  state.people = [];
  state.timeline = [];
  vi.clearAllMocks();
  // A prior test's rejection (see "surfaces a rejected search...") would
  // otherwise leak into every test that runs after it, since clearAllMocks
  // wipes call history but not a mock's configured implementation.
  recordMocks.SearchCandidates.mockResolvedValue(state.candidates);
});

describe("CrmPanel", () => {
  it("lists candidates from the filter search by default", async () => {
    render(() => <CrmPanel />);
    await screen.findByText("Alice Amber");
    expect(screen.getByText("Bob Blue")).toBeTruthy();
    expect(recordMocks.SearchCandidates).toHaveBeenCalled();
  });

  it("passes the typed filter to the backend", async () => {
    render(() => <CrmPanel />);
    await screen.findByText("Alice Amber");
    fireEvent.input(screen.getByLabelText("Filter"), { target: { value: "sydney" } });
    fireEvent.submit(screen.getByLabelText("Filter form"));
    await waitFor(() =>
      expect(recordMocks.SearchCandidates).toHaveBeenLastCalledWith(
        expect.objectContaining({ text: "sydney" }),
      ),
    );
  });

  it("talent search shows ranked people with their why", async () => {
    state.people = [
      {
        candidate: { id: 1, fullName: "Alice Amber" },
        chunkId: 7,
        artifactName: "alice-resume",
        snippet: "Deep quokkastack experience.",
      },
    ];
    render(() => <CrmPanel />);
    fireEvent.input(await screen.findByLabelText("Talent search"), { target: { value: "quokkastack" } });
    fireEvent.submit(screen.getByLabelText("Talent search form"));
    await screen.findByText(/Deep quokkastack experience/);
    expect(searchMocks.People).toHaveBeenCalledWith("quokkastack", 20);
  });

  it("switching record type re-queries that type", async () => {
    render(() => <CrmPanel />);
    await screen.findByText("Alice Amber");
    fireEvent.click(screen.getByRole("tab", { name: "Companies" }));
    await waitFor(() => expect(recordMocks.SearchCompanies).toHaveBeenCalled());
  });

  it("surfaces a rejected search as a verbatim alert", async () => {
    // reloader retries a failed reload once (see latestOnly), so both the
    // first attempt and its retry must fail for the error to surface.
    recordMocks.SearchCandidates.mockRejectedValue(new Error("candidates are unavailable"));
    render(() => <CrmPanel />);
    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toBe("candidates are unavailable");
  });

  it("shows the timeline for the selected record and logs a new interaction", async () => {
    state.timeline = [
      { id: 4, kind: "call", note: "Talked availability.", occurredAt: "2026-08-20", roleTitle: "", initiativeName: "" },
    ];
    render(() => <CrmPanel />);
    fireEvent.click(await screen.findByText("Alice Amber"));
    await screen.findByText(/Talked availability/);

    fireEvent.input(screen.getByLabelText("Interaction note"), { target: { value: "Sent the brief." } });
    fireEvent.submit(screen.getByLabelText("Log interaction form"));
    await waitFor(() =>
      expect(interactionMocks.Log).toHaveBeenCalledWith(
        expect.objectContaining({ targetType: "candidate", targetId: 1, note: "Sent the brief." }),
      ),
    );
    await waitFor(() => expect(interactionMocks.Timeline).toHaveBeenCalledTimes(2));
  });

  it("an outcome kind requires picking a role in the form", async () => {
    render(() => <CrmPanel />);
    fireEvent.click(await screen.findByText("Alice Amber"));
    await screen.findByLabelText("Interaction kind");
    fireEvent.change(screen.getByLabelText("Interaction kind"), { target: { value: "placement" } });
    await screen.findByLabelText("Interaction role");
  });

  it("edits the selected record through the details form", async () => {
    render(() => <CrmPanel />);
    fireEvent.click(await screen.findByText("Alice Amber"));
    const name = await screen.findByLabelText("Full name *");
    fireEvent.input(name, { target: { value: "Alice A. Amber" } });
    fireEvent.submit(screen.getByLabelText("Details form"));
    await waitFor(() =>
      expect(recordMocks.UpdateCandidate).toHaveBeenCalledWith(
        expect.objectContaining({ id: 1, fullName: "Alice A. Amber" }),
      ),
    );
  });
});
