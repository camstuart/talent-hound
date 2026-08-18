import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@solidjs/testing-library";
import ShortlistPanel from "./ShortlistPanel";

// The Go backend is not running: bindings are mocked. Fixtures are invented.
const { state, shortlistMocks, recordMocks } = vi.hoisted(() => {
  const state = {
    shortlist: null as Record<string, unknown> | null,
    buildError: "",
  };
  return {
    state,
    recordMocks: { ListCandidates: vi.fn(async () => [{ id: 1, fullName: "Kalinda Reyes" }]) },
    shortlistMocks: {
      Build: vi.fn(async () => {
        if (state.buildError) throw new Error(state.buildError);
        return state.shortlist;
      }),
    },
  };
});

vi.mock("../../bindings/camstuart/talent-hound", () => ({
  ShortlistService: shortlistMocks,
  RecordService: recordMocks,
}));

const anEntry = (over: Record<string, unknown> = {}) => ({
  roleId: 1,
  title: "Platform engineer",
  position: 1,
  score: 0.032,
  why: [{ source: "five years of production Go", method: "lexical", rank: 1 }],
  conflicts: [],
  ...over,
});

const aShortlist = (over: Record<string, unknown> = {}) => ({
  initiativeId: 1,
  candidateId: 0,
  entries: [anEntry()],
  criteriaVersion: 4,
  spaceId: 1,
  eligible: 6,
  ...over,
});

beforeEach(() => {
  state.shortlist = aShortlist();
  state.buildError = "";
  vi.clearAllMocks();
});

const buildList = async () => {
  const button = await screen.findByLabelText("Build the shortlist");
  await waitFor(() => expect((button as HTMLButtonElement).disabled).toBe(false));
  fireEvent.click(button);
};

describe("ShortlistPanel", () => {
  it("shows the roles in order with the scope they came from", async () => {
    state.shortlist = aShortlist({
      entries: [anEntry(), anEntry({ roleId: 2, title: "Staff engineer", position: 2 })],
    });
    render(() => <ShortlistPanel initiativeId={1} />);
    await buildList();

    await screen.findByText("1. Platform engineer");
    await screen.findByText("2. Staff engineer");
    const scope = await screen.findByLabelText("Shortlist scope");
    expect(scope.textContent).toContain("2 of 6 roles in scope");
    expect(scope.textContent).toContain("criteria version 4");
  });

  it("says why each role is on the list, naming the criterion and the method", async () => {
    state.shortlist = aShortlist({
      entries: [
        anEntry({
          why: [
            { source: "five years of production Go", method: "lexical", rank: 1 },
            { source: "has led a platform team", method: "semantic", rank: 3 },
          ],
        }),
      ],
    });
    render(() => <ShortlistPanel initiativeId={1} />);
    await buildList();

    const why = await screen.findByLabelText("Why Platform engineer is here");
    expect(why.textContent).toContain("lexical match at rank 1");
    expect(why.textContent).toContain("five years of production Go");
    expect(why.textContent).toContain("semantic match at rank 3");
  });

  // A role you would reject is more useful than an empty list.
  it("shows a conflicting role with its conflict rather than hiding it", async () => {
    state.shortlist = aShortlist({
      entries: [
        anEntry({
          conflicts: [{ field: "arrangement", wanted: "remote", found: "onsite" }],
        }),
      ],
    });
    render(() => <ShortlistPanel initiativeId={1} />);
    await buildList();

    // Still listed…
    await screen.findByText("1. Platform engineer");
    // …and the conflict is stated in terms a recruiter can act on.
    const conflict = await screen.findByLabelText("Conflicts for Platform engineer");
    expect(conflict.textContent).toContain("this role says onsite");
    expect(conflict.textContent).toContain("you asked for remote");
  });

  it("says nothing matched without pretending there were no roles", async () => {
    state.shortlist = aShortlist({ entries: [], eligible: 9 });
    render(() => <ShortlistPanel initiativeId={1} />);
    await buildList();

    await screen.findByText(/Nothing matched/);
    // The distinction that matters: roles exist, they just did not match.
    const scope = await screen.findByLabelText("Shortlist scope");
    expect(scope.textContent).toContain("0 of 9 roles in scope");
  });

  it("builds for the chosen candidate", async () => {
    render(() => <ShortlistPanel initiativeId={7} />);
    fireEvent.change(await screen.findByLabelText("Shortlist for candidate"), { target: { value: "1" } });
    await buildList();
    await waitFor(() => expect(shortlistMocks.Build).toHaveBeenCalledWith(7, 1));
  });

  it("shows the backend's own words when a build fails", async () => {
    state.buildError = "nothing is embedded yet for the current model";
    render(() => <ShortlistPanel initiativeId={1} />);
    await buildList();
    await screen.findByText("nothing is embedded yet for the current model");
  });
});
