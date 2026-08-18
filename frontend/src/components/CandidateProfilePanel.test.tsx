import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@solidjs/testing-library";
import CandidateProfilePanel from "./CandidateProfilePanel";

// The Go backend is not running: bindings are mocked. Fixtures are invented.
const { state, profileMocks, recordMocks } = vi.hoisted(() => {
  const state = {
    candidates: [{ id: 1, fullName: "Kalinda Reyes" }] as Record<string, unknown>[],
    profile: null as Record<string, unknown> | null,
    readiness: null as Record<string, unknown> | null,
    citations: [] as Record<string, unknown>[],
    diff: null as Record<string, unknown> | null,
    approveError: "",
  };
  return {
    state,
    recordMocks: { ListCandidates: vi.fn(async () => state.candidates) },
    profileMocks: {
      InUse: vi.fn(async () => state.profile),
      Readiness: vi.fn(async () => state.readiness),
      Citations: vi.fn(async () => state.citations),
      Classify: vi.fn(async () => state.profile ?? { id: 9 }),
      Approve: vi.fn(async () => {
        if (state.approveError) throw new Error(state.approveError);
        return state.profile;
      }),
      EditAspect: vi.fn(async () => state.profile),
      RemoveAspect: vi.fn(async () => state.profile),
      DiffAgainstApproved: vi.fn(async () => state.diff),
      ResolveConflicts: vi.fn(async () => state.profile),
    },
  };
});

vi.mock("../../bindings/camstuart/talent-hound", () => ({
  CandidateProfileService: profileMocks,
  RecordService: recordMocks,
}));

// The panel disables its actions while a load is in flight, so a click has to
// wait for the initial load to settle — otherwise the test races the mount.
const clickWhenReady = async (label: string) => {
  const button = await screen.findByLabelText(label);
  await waitFor(() => expect((button as HTMLButtonElement).disabled).toBe(false));
  fireEvent.click(button);
};

const anAspect = (over: Record<string, unknown> = {}) => ({
  id: 1,
  profileId: 5,
  ordinal: 0,
  type: "skill",
  wording: "Go and SQLite in production",
  structured: "{}",
  priority: "unspecified",
  origin: "extracted",
  citations: "[]",
  ...over,
});

const aProfile = (over: Record<string, unknown> = {}) => ({
  id: 5,
  subjectKind: "candidate",
  subjectId: 1,
  version: 1,
  state: "proposed",
  aspects: [anAspect()],
  ...over,
});

beforeEach(() => {
  state.candidates = [{ id: 1, fullName: "Kalinda Reyes" }];
  state.profile = null;
  state.readiness = null;
  state.citations = [];
  state.diff = null;
  state.approveError = "";
  vi.clearAllMocks();
});

describe("CandidateProfilePanel", () => {
  it("says a candidate has no profile and that matching is blocked", async () => {
    state.readiness = { candidateId: 1, ready: false, reason: "this candidate has no profile yet" };
    render(() => <CandidateProfilePanel initiativeId={1} />);
    await screen.findByText("no profile yet");
    expect(await screen.findByLabelText("Why this candidate is blocked")).toBeTruthy();
  });

  it("shows each aspect with its type and origin", async () => {
    state.profile = aProfile({
      aspects: [anAspect(), anAspect({ ordinal: 1, type: "location", origin: "recruiter_supplied" })],
    });
    state.readiness = { candidateId: 1, ready: false, reason: "not been approved" };
    render(() => <CandidateProfilePanel initiativeId={1} />);

    await screen.findByText("skill");
    expect(await screen.findByText(/Recruiter supplied/)).toBeTruthy();
  });

  it("shows a proposed profile as not yet approved", async () => {
    state.profile = aProfile();
    state.readiness = { candidateId: 1, ready: false, reason: "this candidate's profile has not been approved yet" };
    render(() => <CandidateProfilePanel initiativeId={1} />);
    await screen.findByText("proposed — not yet approved");
  });

  it("warns that an approved profile is stale without hiding that it is in use", async () => {
    state.profile = aProfile({ state: "approved" });
    state.readiness = {
      candidateId: 1,
      ready: true,
      stale: true,
      warning: "the evidence has changed since this profile was approved — review and approve it again",
    };
    render(() => <CandidateProfilePanel initiativeId={1} />);

    await screen.findByText("approved, but the evidence has changed since");
    expect(await screen.findByLabelText("Profile warning")).toBeTruthy();
    // Still in use: no block message.
    expect(screen.queryByLabelText("Why this candidate is blocked")).toBeNull();
  });

  it("navigates from an aspect to the source wording it came from", async () => {
    state.profile = aProfile();
    state.readiness = { candidateId: 1, ready: true };
    state.citations = [
      {
        ordinal: 0,
        artifactId: 2,
        artifactName: "Reyes resume",
        location: "Reyes resume (section 2)",
        text: "Go and SQLite in production since 2019.",
        record: "",
      },
    ];
    render(() => <CandidateProfilePanel initiativeId={1} />);

    fireEvent.click(await screen.findByLabelText("Show the evidence for aspect 1"));
    await screen.findByText("Reyes resume (section 2)");
    const cited = screen.getByLabelText("Cited text for aspect 1");
    expect(cited.tagName).toBe("PRE");
    expect(cited.textContent).toContain("since 2019");
  });

  it("shows an aspect literally, never as markup", async () => {
    // A document a stranger wrote. Nothing here interprets it.
    state.profile = aProfile({ aspects: [anAspect({ wording: "<script>alert('x')</script> **bold**" })] });
    state.readiness = { candidateId: 1, ready: true };
    render(() => <CandidateProfilePanel initiativeId={1} />);

    const shown = await screen.findByLabelText("Aspect 1 wording");
    expect(shown.tagName).toBe("PRE");
    expect(shown.querySelector("script")).toBeNull();
  });

  it("edits an aspect and saves on Enter", async () => {
    state.profile = aProfile();
    state.readiness = { candidateId: 1, ready: true };
    render(() => <CandidateProfilePanel initiativeId={1} />);

    fireEvent.click(await screen.findByLabelText("Edit aspect 1"));
    const field = await screen.findByLabelText("Wording for aspect 1");
    fireEvent.input(field, { target: { value: "Go, SQLite, and PostgreSQL" } });
    // Keyboard operable: Enter commits without reaching for the mouse.
    fireEvent.keyDown(field, { key: "Enter" });

    await waitFor(() =>
      expect(profileMocks.EditAspect).toHaveBeenCalledWith(1, 0, "Go, SQLite, and PostgreSQL", {}),
    );
  });

  it("abandons an edit on Escape", async () => {
    state.profile = aProfile();
    state.readiness = { candidateId: 1, ready: true };
    render(() => <CandidateProfilePanel initiativeId={1} />);

    fireEvent.click(await screen.findByLabelText("Edit aspect 1"));
    const field = await screen.findByLabelText("Wording for aspect 1");
    fireEvent.keyDown(field, { key: "Escape" });

    await waitFor(() => expect(screen.queryByLabelText("Wording for aspect 1")).toBeNull());
    expect(profileMocks.EditAspect).not.toHaveBeenCalled();
  });

  it("removes an aspect", async () => {
    state.profile = aProfile();
    state.readiness = { candidateId: 1, ready: true };
    render(() => <CandidateProfilePanel initiativeId={1} />);

    fireEvent.click(await screen.findByLabelText("Remove aspect 1"));
    await waitFor(() => expect(profileMocks.RemoveAspect).toHaveBeenCalledWith(1, 0));
  });

  it("approves the profile in use", async () => {
    state.profile = aProfile();
    state.readiness = { candidateId: 1, ready: false, reason: "not approved" };
    render(() => <CandidateProfilePanel initiativeId={1} />);

    await clickWhenReady("Approve this profile");
    await waitFor(() => expect(profileMocks.Approve).toHaveBeenCalledWith(5));
  });

  it("shows the backend's own words when approval is refused", async () => {
    state.profile = aProfile({ state: "failed" });
    state.readiness = { candidateId: 1, ready: false, reason: "could not be built" };
    state.approveError = "a failed profile cannot be approved — retry it, or build one by hand";
    render(() => <CandidateProfilePanel initiativeId={1} />);

    await clickWhenReady("Approve this profile");
    await screen.findByText("a failed profile cannot be approved — retry it, or build one by hand");
  });

  it("reviews a diff and applies only the conflicts the recruiter took", async () => {
    state.profile = aProfile({ id: 5, state: "approved" });
    state.readiness = { candidateId: 1, ready: true };
    state.diff = {
      approvedProfileId: 5,
      proposedProfileId: 6,
      additions: [{ ordinal: 1, type: "location", wording: "Melbourne", origin: "extracted" }],
      removals: [{ ordinal: 2, type: "seniority", wording: "Senior engineer", origin: "extracted" }],
      conflicts: [
        {
          approved: { ordinal: 0, type: "skill", wording: "since 2019", origin: "extracted" },
          proposed: { ordinal: 0, type: "skill", wording: "since 2021", origin: "extracted" },
        },
      ],
    };
    // A reclassification against an approved profile returns a different version.
    profileMocks.Classify.mockResolvedValueOnce({ id: 6 });
    render(() => <CandidateProfilePanel initiativeId={1} />);

    await clickWhenReady("Build this candidate's profile");
    await screen.findByText(/1 added, 1 removed, 1 conflicting/);
    // Both sides of the conflict are shown, so the recruiter is choosing rather
    // than being told.
    expect(await screen.findByLabelText("Approved wording for conflict 1")).toBeTruthy();
    expect(await screen.findByLabelText("Proposed wording for conflict 1")).toBeTruthy();

    fireEvent.click(screen.getByLabelText("Take the proposed wording for conflict 1"));
    fireEvent.click(screen.getByLabelText("Apply the proposed changes"));
    await waitFor(() => expect(profileMocks.ResolveConflicts).toHaveBeenCalledWith(1, 6, [0]));
  });

  it("applies no conflict when the recruiter takes none", async () => {
    state.profile = aProfile({ id: 5, state: "approved" });
    state.readiness = { candidateId: 1, ready: true };
    state.diff = {
      approvedProfileId: 5,
      proposedProfileId: 6,
      additions: [],
      removals: [],
      conflicts: [
        {
          approved: { ordinal: 0, type: "skill", wording: "since 2019", origin: "extracted" },
          proposed: { ordinal: 0, type: "skill", wording: "since 2021", origin: "extracted" },
        },
      ],
    };
    profileMocks.Classify.mockResolvedValueOnce({ id: 6 });
    render(() => <CandidateProfilePanel initiativeId={1} />);

    await clickWhenReady("Build this candidate's profile");
    await screen.findByLabelText("Apply the proposed changes");
    fireEvent.click(screen.getByLabelText("Apply the proposed changes"));
    await waitFor(() => expect(profileMocks.ResolveConflicts).toHaveBeenCalledWith(1, 6, []));
  });
});
