import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@solidjs/testing-library";
import CrmPanel from "./CrmPanel";

// The Go backend is not running: bindings are mocked. Every fixture is invented.
const { state, recordMocks, searchMocks, interactionMocks } = vi.hoisted(() => {
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
      GetCandidate: vi.fn(async (id: number) => state.candidates.find((c) => c.id === id)),
      CreateCandidate: vi.fn(async (c: Record<string, unknown>) => ({ id: 9, ...c })),
      UpdateCandidate: vi.fn(async (c: Record<string, unknown>) => c),
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
  };
});

vi.mock("../../bindings/camstuart/talent-hound", () => ({
  RecordService: recordMocks,
  SearchService: searchMocks,
  InteractionService: interactionMocks,
}));

beforeEach(() => {
  state.people = [];
  state.timeline = [];
  vi.clearAllMocks();
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
});
