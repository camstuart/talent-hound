import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@solidjs/testing-library";
import CriteriaPanel from "./CriteriaPanel";

// The Go backend is not running: bindings are mocked. Fixtures are invented.
const { state, criteriaMocks, recordMocks } = vi.hoisted(() => {
  const state = {
    criteria: [] as Record<string, unknown>[],
    version: 3,
    proposals: [] as Record<string, unknown>[],
    addError: "",
    proposeError: "",
  };
  return {
    state,
    recordMocks: { ListCandidates: vi.fn(async () => [{ id: 1, fullName: "Kalinda Reyes" }]) },
    criteriaMocks: {
      List: vi.fn(async () => state.criteria),
      Version: vi.fn(async () => state.version),
      Add: vi.fn(async () => {
        if (state.addError) throw new Error(state.addError);
        return { id: 9 };
      }),
      Edit: vi.fn(async () => ({ id: 9 })),
      Remove: vi.fn(async () => undefined),
      Reorder: vi.fn(async () => undefined),
      Propose: vi.fn(async () => {
        if (state.proposeError) throw new Error(state.proposeError);
        return state.proposals;
      }),
      Apply: vi.fn(async () => []),
    },
  };
});

vi.mock("../../bindings/camstuart/talent-hound", () => ({
  CriteriaService: criteriaMocks,
  RecordService: recordMocks,
}));

const aCriterion = (over: Record<string, unknown> = {}) => ({
  id: 1,
  initiativeId: 1,
  position: 0,
  text: "five years of production Go",
  priority: "must_have",
  warning: "",
  ...over,
});

beforeEach(() => {
  state.criteria = [];
  state.version = 3;
  state.proposals = [];
  state.addError = "";
  state.proposeError = "";
  vi.clearAllMocks();
});

describe("CriteriaPanel", () => {
  it("shows criteria with their priority and the criteria version", async () => {
    state.criteria = [aCriterion(), aCriterion({ id: 2, priority: "nice_to_have", text: "has led a team" })];
    render(() => <CriteriaPanel initiativeId={1} />);

    await screen.findByText(/version 3/);
    expect(await screen.findByText(/Must have —/)).toBeTruthy();
    expect(await screen.findByText(/Nice to have —/)).toBeTruthy();
  });

  it("adds a criterion with the chosen priority on Enter", async () => {
    render(() => <CriteriaPanel initiativeId={4} />);

    const field = await screen.findByLabelText("New criterion");
    fireEvent.input(field, { target: { value: "five years of production Go" } });
    fireEvent.change(screen.getByLabelText("Priority to add with"), {
      target: { value: "nice_to_have" },
    });
    fireEvent.keyDown(field, { key: "Enter" });

    await waitFor(() =>
      expect(criteriaMocks.Add).toHaveBeenCalledWith({
        initiativeId: 4,
        text: "five years of production Go",
        priority: "nice_to_have",
      }),
    );
  });

  // A refusal is a rule, not a failure, and it says there is no way past it.
  it("shows a deterministic refusal as final", async () => {
    state.addError = 'this criterion names age ("under 35"), which cannot be a search criterion';
    render(() => <CriteriaPanel initiativeId={1} />);

    fireEvent.input(await screen.findByLabelText("New criterion"), { target: { value: "must be under 35" } });
    fireEvent.click(screen.getByLabelText("Add this criterion"));

    const refusal = await screen.findByLabelText("Refused criterion");
    expect(refusal.textContent).toContain("names age");
    expect(refusal.textContent).toContain("no way to add this one");
  });

  // A warning is advice about a criterion that is stored and in use.
  it("shows a proxy warning without hiding that the criterion is in use", async () => {
    state.criteria = [
      aCriterion({ text: "recent graduate preferred", warning: '"recent graduate" tends to select for age' }),
    ];
    render(() => <CriteriaPanel initiativeId={1} />);

    const warning = await screen.findByLabelText("Warning about criterion 1");
    expect(warning.textContent).toContain("Possible proxy");
    // The criterion itself is listed like any other, with its actions.
    expect(await screen.findByLabelText("Criterion 1")).toBeTruthy();
    expect(await screen.findByLabelText("Remove criterion 1")).toBeTruthy();
    // And it is not shown as a refusal.
    expect(screen.queryByLabelText("Refused criterion")).toBeNull();
  });

  it("shows a criterion literally, never as markup", async () => {
    state.criteria = [aCriterion({ text: "<script>alert('x')</script>" })];
    render(() => <CriteriaPanel initiativeId={1} />);

    const shown = await screen.findByLabelText("Criterion 1");
    expect(shown.tagName).toBe("PRE");
    expect(shown.querySelector("script")).toBeNull();
  });

  it("edits a criterion and saves on Enter", async () => {
    state.criteria = [aCriterion()];
    render(() => <CriteriaPanel initiativeId={1} />);

    fireEvent.click(await screen.findByLabelText("Edit criterion 1"));
    const field = await screen.findByLabelText("Wording for criterion 1");
    fireEvent.input(field, { target: { value: "six years of production Go" } });
    fireEvent.keyDown(field, { key: "Enter" });

    await waitFor(() => expect(criteriaMocks.Edit).toHaveBeenCalledWith(1, "six years of production Go", "must_have"));
  });

  it("changes a criterion's priority", async () => {
    state.criteria = [aCriterion()];
    render(() => <CriteriaPanel initiativeId={1} />);

    fireEvent.click(await screen.findByLabelText("Make criterion 1 nice to have"));
    await waitFor(() =>
      expect(criteriaMocks.Edit).toHaveBeenCalledWith(1, "five years of production Go", "nice_to_have"),
    );
  });

  it("reorders criteria without touching their content", async () => {
    state.criteria = [aCriterion(), aCriterion({ id: 2, text: "has led a team" })];
    render(() => <CriteriaPanel initiativeId={7} />);

    fireEvent.click(await screen.findByLabelText("Move criterion 1 down"));
    await waitFor(() => expect(criteriaMocks.Reorder).toHaveBeenCalledWith(7, [2, 1]));
    // Reordering is not an edit.
    expect(criteriaMocks.Edit).not.toHaveBeenCalled();
  });

  it("removes a criterion", async () => {
    state.criteria = [aCriterion()];
    render(() => <CriteriaPanel initiativeId={1} />);
    fireEvent.click(await screen.findByLabelText("Remove criterion 1"));
    await waitFor(() => expect(criteriaMocks.Remove).toHaveBeenCalledWith(1));
  });

  // Nothing proposed becomes a criterion without a person choosing it.
  it("applies only the proposals the recruiter chose", async () => {
    state.proposals = [
      { text: "Go and SQLite in production", priority: "nice_to_have", from: "skill" },
      { text: "Senior platform engineer", priority: "nice_to_have", from: "seniority" },
    ];
    render(() => <CriteriaPanel initiativeId={2} />);

    fireEvent.click(await screen.findByLabelText("Propose criteria from this candidate's approved profile"));
    await screen.findByText(/Nothing here is a criterion until you apply it/);
    // Proposing on its own writes nothing.
    expect(criteriaMocks.Apply).not.toHaveBeenCalled();

    fireEvent.click(screen.getByLabelText("Apply proposal 2"));
    fireEvent.click(screen.getByLabelText("Apply the chosen proposals"));

    await waitFor(() =>
      expect(criteriaMocks.Apply).toHaveBeenCalledWith(2, [
        { text: "Senior platform engineer", priority: "nice_to_have", from: "seniority" },
      ]),
    );
  });

  it("applies nothing when no proposal is chosen", async () => {
    state.proposals = [{ text: "Go and SQLite in production", priority: "nice_to_have", from: "skill" }];
    render(() => <CriteriaPanel initiativeId={2} />);

    fireEvent.click(await screen.findByLabelText("Propose criteria from this candidate's approved profile"));
    await screen.findByLabelText("Apply the chosen proposals");
    fireEvent.click(screen.getByLabelText("Apply the chosen proposals"));

    await waitFor(() => expect(criteriaMocks.Apply).not.toHaveBeenCalled());
  });

  it("shows the backend's own words when proposals need an approved profile", async () => {
    state.proposeError = "criteria can only be proposed from an approved profile: this candidate has no profile yet";
    render(() => <CriteriaPanel initiativeId={1} />);

    fireEvent.click(await screen.findByLabelText("Propose criteria from this candidate's approved profile"));
    await screen.findByText(/criteria can only be proposed from an approved profile/);
  });
});
