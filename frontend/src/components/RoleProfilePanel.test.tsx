import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@solidjs/testing-library";
import RoleProfilePanel from "./RoleProfilePanel";

// The Go backend is not running: bindings are mocked. Fixtures are invented.
const { state, roleMocks, recordMocks } = vi.hoisted(() => {
  const state = {
    statuses: [] as Record<string, unknown>[],
    roles: [{ id: 1, title: "Senior platform engineer" }] as Record<string, unknown>[],
    citations: [] as Record<string, unknown>[],
    profileError: "",
  };
  return {
    state,
    recordMocks: { ListRoles: vi.fn(async () => state.roles) },
    roleMocks: {
      List: vi.fn(async () => state.statuses),
      Profile: vi.fn(async () => {
        if (state.profileError) throw new Error(state.profileError);
        return { id: 5 };
      }),
      Citations: vi.fn(async () => state.citations),
      EditAspect: vi.fn(async () => ({ id: 6 })),
      RemoveAspect: vi.fn(async () => ({ id: 7 })),
      AddAspect: vi.fn(async () => ({ id: 8 })),
    },
  };
});

vi.mock("../../bindings/camstuart/talent-hound", () => ({
  RoleProfileService: roleMocks,
  RecordService: recordMocks,
}));

const anAspect = (over: Record<string, unknown> = {}) => ({
  id: 1,
  profileId: 5,
  ordinal: 0,
  type: "skill",
  wording: "Strong Go and production SQLite",
  structured: "{}",
  priority: "must_have",
  origin: "extracted",
  citations: "[]",
  ...over,
});

const aStatus = (over: Record<string, unknown> = {}) => ({
  roleId: 1,
  profileId: 5,
  state: "ready",
  reason: "",
  aspects: [anAspect()],
  ...over,
});

beforeEach(() => {
  state.statuses = [];
  state.roles = [{ id: 1, title: "Senior platform engineer" }];
  state.citations = [];
  state.profileError = "";
  vi.clearAllMocks();
});

describe("RoleProfilePanel", () => {
  it("labels a ready profile as used in assessment", async () => {
    state.statuses = [aStatus()];
    render(() => <RoleProfilePanel />);
    await screen.findByText(/ready — used in assessment/);
  });

  it("keeps a failed profile visible with its reason and a way forward", async () => {
    state.statuses = [
      aStatus({
        state: "failed",
        reason: "this listing could not be decomposed (invalid_proposal) — retry it, or enter the requirements by hand",
        aspects: [],
      }),
    ];
    render(() => <RoleProfilePanel />);

    // Both the label and the reason say it; the label is what the row leads with.
    await screen.findByLabelText("Why Senior platform engineer is not assessed");
    expect(screen.getAllByText(/could not be decomposed/)).toHaveLength(2);
    // A failure that vanishes is indistinguishable from a role never
    // discovered, so both routes out of it are on screen.
    expect(await screen.findByLabelText("Profile Senior platform engineer")).toBeTruthy();
    expect(await screen.findByLabelText("Add a requirement to Senior platform engineer")).toBeTruthy();
  });

  it("keeps a stale profile visible and says the listing changed", async () => {
    state.statuses = [
      aStatus({ state: "stale", reason: "the listing has changed since this profile was made — profile it again" }),
    ];
    render(() => <RoleProfilePanel />);
    await screen.findByText(/the listing changed since this was made/);
    await screen.findByText(/profile it again/);
  });

  it("shows a role that has never been profiled rather than omitting it", async () => {
    state.statuses = [aStatus({ state: "unprofiled", reason: "this role has not been profiled yet", aspects: [] })];
    render(() => <RoleProfilePanel />);
    await screen.findByText(/not profiled yet/);
    expect(await screen.findByText("Profile this listing")).toBeTruthy();
  });

  it("shows each requirement with its priority and origin", async () => {
    state.statuses = [
      aStatus({
        aspects: [anAspect(), anAspect({ ordinal: 1, priority: "unspecified", origin: "recruiter_supplied" })],
      }),
    ];
    render(() => <RoleProfilePanel />);

    await screen.findByText(/must have, extracted/);
    await screen.findByText(/unspecified, Recruiter supplied/);
  });

  it("shows a requirement literally, never as markup", async () => {
    // A listing a stranger wrote. Nothing here interprets it.
    state.statuses = [aStatus({ aspects: [anAspect({ wording: "<script>alert('x')</script>" })] })];
    render(() => <RoleProfilePanel />);

    const shown = await screen.findByLabelText("Requirement 1 of Senior platform engineer");
    expect(shown.tagName).toBe("PRE");
    expect(shown.querySelector("script")).toBeNull();
  });

  it("navigates from a requirement to the listing wording it came from", async () => {
    state.statuses = [aStatus()];
    state.citations = [
      {
        ordinal: 0,
        artifactId: 2,
        artifactName: "listing.md",
        location: "listing.md (section 2)",
        text: "You must have strong Go and production SQLite experience.",
        record: "",
      },
    ];
    render(() => <RoleProfilePanel />);

    fireEvent.click(await screen.findByLabelText("Show the evidence for requirement 1 of Senior platform engineer"));
    await screen.findByText("listing.md (section 2)");
    const cited = screen.getByLabelText("Cited listing text for requirement 1");
    expect(cited.tagName).toBe("PRE");
  });

  it("edits a requirement and saves on Enter, keeping its priority", async () => {
    state.statuses = [aStatus()];
    render(() => <RoleProfilePanel />);

    fireEvent.click(await screen.findByLabelText("Edit requirement 1 of Senior platform engineer"));
    const field = await screen.findByLabelText("Wording for requirement 1 of Senior platform engineer");
    fireEvent.input(field, { target: { value: "Go and SQLite — five years stated, negotiable" } });
    fireEvent.keyDown(field, { key: "Enter" });

    await waitFor(() =>
      expect(roleMocks.EditAspect).toHaveBeenCalledWith(1, 0, "Go and SQLite — five years stated, negotiable", "must_have"),
    );
  });

  it("abandons an edit on Escape", async () => {
    state.statuses = [aStatus()];
    render(() => <RoleProfilePanel />);

    fireEvent.click(await screen.findByLabelText("Edit requirement 1 of Senior platform engineer"));
    const field = await screen.findByLabelText("Wording for requirement 1 of Senior platform engineer");
    fireEvent.keyDown(field, { key: "Escape" });

    await waitFor(() =>
      expect(screen.queryByLabelText("Wording for requirement 1 of Senior platform engineer")).toBeNull(),
    );
    expect(roleMocks.EditAspect).not.toHaveBeenCalled();
  });

  it("removes a requirement", async () => {
    state.statuses = [aStatus()];
    render(() => <RoleProfilePanel />);
    fireEvent.click(await screen.findByLabelText("Remove requirement 1 of Senior platform engineer"));
    await waitFor(() => expect(roleMocks.RemoveAspect).toHaveBeenCalledWith(1, 0));
  });

  it("adds a requirement by hand to a failed profile", async () => {
    state.statuses = [aStatus({ state: "failed", reason: "could not be decomposed", aspects: [] })];
    render(() => <RoleProfilePanel />);

    const field = await screen.findByLabelText("Requirement to add to Senior platform engineer");
    fireEvent.input(field, { target: { value: "Must have Go" } });
    fireEvent.click(screen.getByLabelText("Add a requirement to Senior platform engineer"));

    await waitFor(() => expect(roleMocks.AddAspect).toHaveBeenCalled());
    const [roleId, aspect] = roleMocks.AddAspect.mock.calls[0] as unknown as [number, Record<string, unknown>];
    expect(roleId).toBe(1);
    expect(aspect.wording).toBe("Must have Go");
    expect(aspect.origin).toBe("recruiter_supplied");
  });

  it("shows the backend's own words when profiling fails", async () => {
    state.statuses = [aStatus({ state: "unprofiled", reason: "not profiled yet", aspects: [] })];
    state.profileError = "no model resolves for the classify role — assign classify or generate";
    render(() => <RoleProfilePanel />);

    fireEvent.click(await screen.findByLabelText("Profile Senior platform engineer"));
    await screen.findByText("no model resolves for the classify role — assign classify or generate");
  });
});
