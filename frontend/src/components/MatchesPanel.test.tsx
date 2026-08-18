import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@solidjs/testing-library";
import MatchesPanel from "./MatchesPanel";

// The Go backend is not running: bindings are mocked. Fixtures are invented.
const { state, assessMocks, recordMocks } = vi.hoisted(() => {
  const state = {
    matches: [] as Record<string, unknown>[],
    assessError: "",
  };
  return {
    state,
    recordMocks: { ListCandidates: vi.fn(async () => [{ id: 1, fullName: "Kalinda Reyes" }]) },
    assessMocks: {
      Matches: vi.fn(async () => state.matches),
      AssessAll: vi.fn(async () => {
        if (state.assessError) throw new Error(state.assessError);
        return { id: 4 };
      }),
    },
  };
});

vi.mock("../../bindings/camstuart/talent-hound", () => ({
  AssessService: assessMocks,
  RecordService: recordMocks,
}));

const aResult = (over: Record<string, unknown> = {}) => ({
  id: 1,
  matchId: 1,
  direction: "candidate_fits_role",
  ordinal: 0,
  requirement: "Strong Go",
  priority: "must_have",
  result: "met",
  reason: "five years of production Go",
  citations: JSON.stringify([{ ref: "candidate-aspect-1", text: "Five years of production Go" }]),
  ...over,
});

const aMatch = (over: Record<string, unknown> = {}) => ({
  id: 1,
  initiativeId: 1,
  candidateId: 1,
  roleId: 5,
  roleTitle: "Platform engineer",
  inputHash: "abc",
  unmetMustHaves: 0,
  unknownMustHaves: 0,
  metNiceToHaves: 2,
  retrievalPosition: 1,
  failureReason: "",
  assessedAt: "2026-03-01T12:00:00Z",
  stale: false,
  results: [aResult()],
  ...over,
});

beforeEach(() => {
  state.matches = [aMatch()];
  state.assessError = "";
  vi.clearAllMocks();
});

const chooseCandidate = async () => {
  fireEvent.change(await screen.findByLabelText("Matches for candidate"), { target: { value: "1" } });
  await waitFor(() => expect(assessMocks.Matches).toHaveBeenCalled());
};

describe("MatchesPanel", () => {
  it("shows each match with its tallies in order", async () => {
    state.matches = [
      aMatch(),
      aMatch({ id: 2, roleId: 6, roleTitle: "Staff engineer", unmetMustHaves: 2, metNiceToHaves: 0 }),
    ];
    render(() => <MatchesPanel initiativeId={1} />);
    await chooseCandidate();

    await screen.findByText(/1\. Platform engineer/);
    await screen.findByText(/2\. Staff engineer/);
    // The failing one is listed, not hidden.
    expect(await screen.findByText(/2 unmet must-haves/)).toBeTruthy();
  });

  it("labels both directions distinctly", async () => {
    state.matches = [
      aMatch({
        results: [
          aResult({ direction: "role_fits_candidate", requirement: "hybrid in Melbourne" }),
          aResult({ direction: "candidate_fits_role", requirement: "Strong Go" }),
        ],
      }),
    ];
    render(() => <MatchesPanel initiativeId={1} />);
    await chooseCandidate();

    fireEvent.click(await screen.findByLabelText("Show the assessment of match 1"));
    await screen.findByText("Does this role suit them");
    await screen.findByText("Do they suit this role");
  });

  it("shows met, not met, and no-evidence distinctly", async () => {
    state.matches = [
      aMatch({
        results: [
          aResult({ result: "met" }),
          aResult({ ordinal: 1, result: "not_met", requirement: "Ten years of Erlang" }),
          aResult({ ordinal: 2, result: "unknown", requirement: "A postgraduate qualification", citations: "[]" }),
        ],
      }),
    ];
    render(() => <MatchesPanel initiativeId={1} />);
    await chooseCandidate();
    fireEvent.click(await screen.findByLabelText("Show the assessment of match 1"));

    await screen.findByText(/^met$/);
    await screen.findByText(/^not met$/);
    // An unknown says what it means rather than showing a bare word.
    await screen.findByText("no evidence either way");
  });

  it("navigates from a result to the evidence it cites", async () => {
    render(() => <MatchesPanel initiativeId={1} />);
    await chooseCandidate();
    fireEvent.click(await screen.findByLabelText("Show the assessment of match 1"));

    fireEvent.click(await screen.findByLabelText("Show the evidence for requirement 1 of Do they suit this role"));
    const cited = await screen.findByLabelText("Cited evidence candidate-aspect-1");
    expect(cited.tagName).toBe("PRE");
    expect(cited.textContent).toContain("Five years of production Go");
  });

  it("offers no evidence button when a result cites nothing", async () => {
    state.matches = [aMatch({ results: [aResult({ result: "unknown", citations: "[]" })] })];
    render(() => <MatchesPanel initiativeId={1} />);
    await chooseCandidate();
    fireEvent.click(await screen.findByLabelText("Show the assessment of match 1"));

    await screen.findByText("no evidence either way");
    expect(
      screen.queryByLabelText("Show the evidence for requirement 1 of Do they suit this role"),
    ).toBeNull();
  });

  it("shows a requirement literally, never as markup", async () => {
    state.matches = [aMatch({ results: [aResult({ requirement: "<script>alert('x')</script>" })] })];
    render(() => <MatchesPanel initiativeId={1} />);
    await chooseCandidate();
    fireEvent.click(await screen.findByLabelText("Show the assessment of match 1"));

    const shown = await screen.findByLabelText("Requirement 1 of Do they suit this role");
    expect(shown.tagName).toBe("PRE");
    expect(shown.querySelector("script")).toBeNull();
  });

  // A stale match that looks current is the quiet drift the hash exists to
  // prevent.
  it("says when a match's inputs have changed", async () => {
    state.matches = [aMatch({ stale: true })];
    render(() => <MatchesPanel initiativeId={1} />);
    await chooseCandidate();

    const warning = await screen.findByLabelText("Staleness of match 1");
    expect(warning.textContent).toContain("has changed");
    expect(warning.textContent).toContain("assess again");
  });

  it("assesses the shortlist for the chosen candidate", async () => {
    render(() => <MatchesPanel initiativeId={9} />);
    await chooseCandidate();
    const button = await screen.findByLabelText("Assess the shortlist");
    await waitFor(() => expect((button as HTMLButtonElement).disabled).toBe(false));
    fireEvent.click(button);
    await waitFor(() => expect(assessMocks.AssessAll).toHaveBeenCalledWith(9, 1));
  });

  it("shows the backend's own words when assessment is refused", async () => {
    state.assessError = "no model resolves for the generate role";
    render(() => <MatchesPanel initiativeId={1} />);
    await chooseCandidate();
    const button = await screen.findByLabelText("Assess the shortlist");
    await waitFor(() => expect((button as HTMLButtonElement).disabled).toBe(false));
    fireEvent.click(button);
    await screen.findByText("no model resolves for the generate role");
  });
});
