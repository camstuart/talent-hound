import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@solidjs/testing-library";
import DraftsPanel from "./DraftsPanel";

// The Go backend is not running: bindings are mocked. Fixtures are invented.
const { state, draftMocks, qaMocks, recordMocks } = vi.hoisted(() => {
  const state = {
    answers: [] as Record<string, unknown>[],
    drafts: [] as Record<string, unknown>[],
    askError: "",
    generateError: "",
  };
  return {
    state,
    recordMocks: { ListCandidates: vi.fn(async () => [{ id: 1, fullName: "Kalinda Reyes" }]) },
    qaMocks: {
      Answers: vi.fn(async () => state.answers),
      Ask: vi.fn(async () => {
        if (state.askError) throw new Error(state.askError);
        return { id: 1 };
      }),
    },
    draftMocks: {
      Drafts: vi.fn(async () => state.drafts),
      Generate: vi.fn(async () => {
        if (state.generateError) throw new Error(state.generateError);
        return { id: 2 };
      }),
      Edit: vi.fn(async () => ({ id: 2 })),
      Copy: vi.fn(async () => undefined),
      Discard: vi.fn(async () => undefined),
    },
  };
});

vi.mock("../../bindings/camstuart/talent-hound", () => ({
  DraftService: draftMocks,
  QAService: qaMocks,
  RecordService: recordMocks,
}));

const anAnswer = (over: Record<string, unknown> = {}) => ({
  id: 1,
  initiativeId: 1,
  question: "what does the brief ask for",
  answer: "quokkastack experience",
  supported: true,
  citations: JSON.stringify([
    { ref: "evidence-1", text: "We need quokkastack experience.", location: "brief (section 2)" },
  ]),
  askedAt: "2026-03-01T12:00:00Z",
  proposals: [],
  ...over,
});

const aDraft = (over: Record<string, unknown> = {}) => ({
  id: 2,
  initiativeId: 1,
  kind: "pitch",
  state: "active",
  subject: "A platform engineer worth meeting",
  body: "They have five years of production Go.",
  claims: JSON.stringify([{ text: "five years of production Go", refs: ["profile-1"] }]),
  copies: 0,
  ...over,
});

beforeEach(() => {
  state.answers = [];
  state.drafts = [];
  state.askError = "";
  state.generateError = "";
  vi.clearAllMocks();
});

// The candidate list loads asynchronously, so a select change before the
// options exist does not stick.
const chooseCandidate = async () => {
  const select = await screen.findByLabelText("Draft about candidate");
  await waitFor(() => expect((select as HTMLSelectElement).options.length).toBeGreaterThan(1));
  fireEvent.change(select, { target: { value: "1" } });
};

const clickWhenReady = async (label: string) => {
  const button = await screen.findByLabelText(label);
  await waitFor(() => expect((button as HTMLButtonElement).disabled).toBe(false));
  fireEvent.click(button);
};

describe("DraftsPanel", () => {
  it("asks a question on Enter", async () => {
    render(() => <DraftsPanel initiativeId={4} />);
    const field = await screen.findByLabelText("Question");
    fireEvent.input(field, { target: { value: "what does the brief ask for" } });
    fireEvent.keyDown(field, { key: "Enter" });
    await waitFor(() => expect(qaMocks.Ask).toHaveBeenCalledWith(4, "what does the brief ask for"));
  });

  it("navigates from an answer to the evidence it cites", async () => {
    state.answers = [anAnswer()];
    render(() => <DraftsPanel initiativeId={1} />);

    await screen.findByText(/supported by evidence/);
    fireEvent.click(await screen.findByLabelText("Show the evidence for answer 1"));
    await screen.findByText("brief (section 2)");
    const cited = await screen.findByLabelText("Cited evidence for answer 1");
    expect(cited.tagName).toBe("PRE");
  });

  // The alternative to "I cannot find that" is invention.
  it("shows an unsupported answer as unsupported", async () => {
    state.answers = [
      anAnswer({ supported: false, answer: "the evidence in this initiative does not say", citations: "[]" }),
    ];
    render(() => <DraftsPanel initiativeId={1} />);

    await screen.findByText(/not supported/);
    await screen.findByText("the evidence in this initiative does not say");
    // Nothing to navigate to, so no evidence button.
    expect(screen.queryByLabelText("Show the evidence for answer 1")).toBeNull();
  });

  // A suggestion is a suggestion until a person applies it.
  it("shows suggestions as suggestions rather than applying them", async () => {
    state.answers = [anAnswer({ proposals: ["five years of production Go"] })];
    render(() => <DraftsPanel initiativeId={1} />);

    const note = await screen.findByLabelText("Suggestions from answer 1");
    expect(note.textContent).toContain("add them yourself");
    expect(note.textContent).toContain("five years of production Go");
  });

  it("generates a pitch for the chosen candidate", async () => {
    render(() => <DraftsPanel initiativeId={7} />);
    await chooseCandidate();
    await clickWhenReady("Write a pitch");
    await waitFor(() =>
      expect(draftMocks.Generate).toHaveBeenCalledWith({
        initiativeId: 7,
        candidateId: 1,
        roleId: 0,
        kind: "pitch",
      }),
    );
  });

  it("shows a draft with what it rests on", async () => {
    state.drafts = [aDraft()];
    render(() => <DraftsPanel initiativeId={1} />);

    const body = await screen.findByLabelText("Draft 1");
    expect(body.tagName).toBe("PRE");
    const claims = await screen.findByLabelText("What draft 1 rests on");
    expect(claims.textContent).toContain("five years of production Go");
    expect(claims.textContent).toContain("profile-1");
  });

  it("edits a draft and abandons on Escape", async () => {
    state.drafts = [aDraft()];
    render(() => <DraftsPanel initiativeId={1} />);

    fireEvent.click(await screen.findByLabelText("Edit draft 1"));
    const field = await screen.findByLabelText("Text of draft 1");
    fireEvent.keyDown(field, { key: "Escape" });
    await waitFor(() => expect(screen.queryByLabelText("Text of draft 1")).toBeNull());
    expect(draftMocks.Edit).not.toHaveBeenCalled();

    fireEvent.click(await screen.findByLabelText("Edit draft 1"));
    const again = await screen.findByLabelText("Text of draft 1");
    fireEvent.input(again, { target: { value: "My own words." } });
    fireEvent.click(screen.getByLabelText("Save draft 1"));
    await waitFor(() => expect(draftMocks.Edit).toHaveBeenCalledWith(2, "", "My own words."));
  });

  it("copies a draft and confirms it", async () => {
    state.drafts = [aDraft()];
    render(() => <DraftsPanel initiativeId={1} />);

    fireEvent.click(await screen.findByLabelText("Copy draft 1"));
    await waitFor(() => expect(draftMocks.Copy).toHaveBeenCalledWith(2));
    const confirmation = await screen.findByLabelText("Copy confirmation for draft 1");
    // The confirmation says what happened — a copy, not a send.
    expect(confirmation.textContent).toContain("paste it wherever you send from");
    expect(confirmation.textContent).not.toMatch(/\bsent\b/);
  });

  it("shows the copy count", async () => {
    state.drafts = [aDraft({ copies: 2 })];
    render(() => <DraftsPanel initiativeId={1} />);
    await screen.findByText(/copied 2 times/);
  });

  it("discards a draft and offers no actions afterwards", async () => {
    state.drafts = [aDraft({ state: "discarded" })];
    render(() => <DraftsPanel initiativeId={1} />);

    await screen.findByText(/discarded/);
    expect(screen.queryByLabelText("Copy draft 1")).toBeNull();
    expect(screen.queryByLabelText("Edit draft 1")).toBeNull();
    expect(screen.queryByLabelText("Discard draft 1")).toBeNull();
  });

  it("offers nothing that sends", async () => {
    state.drafts = [aDraft()];
    render(() => <DraftsPanel initiativeId={1} />);
    await screen.findByLabelText("Draft 1");

    for (const forbidden of [/^send$/i, /send message/i, /send email/i]) {
      expect(screen.queryByRole("button", { name: forbidden })).toBeNull();
    }
  });

  it("shows the backend's own words when drafting is refused", async () => {
    state.generateError = "a draft is written from approved evidence: this candidate has no profile yet";
    render(() => <DraftsPanel initiativeId={1} />);
    await chooseCandidate();
    await clickWhenReady("Write a pitch");
    await screen.findByText(/a draft is written from approved evidence/);
  });
});
