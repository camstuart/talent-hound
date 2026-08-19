import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@solidjs/testing-library";
import HelpPanel from "./HelpPanel";

// The Go backend is not running: bindings are mocked. Every fixture is invented.
const { state, mocks } = vi.hoisted(() => {
  const state = { answer: null as Record<string, unknown> | null };
  return {
    state,
    mocks: {
      HelpService: {
        Topics: vi.fn(async () => [
          {
            group: "First steps",
            topics: [
              { id: "getting-started", title: "Getting started", group: "First steps", summary: "What it needs." },
              { id: "tutorial", title: "Tutorial", group: "First steps", summary: "The whole loop." },
            ],
          },
          {
            group: "Rules",
            topics: [
              { id: "deleting-things", title: "Deleting things", group: "Rules", summary: "What blocks a deletion." },
            ],
          },
        ]),
        Article: vi.fn(async (id: string) => ({
          id,
          title: id === "tutorial" ? "Tutorial" : "Getting started",
          group: "First steps",
          summary: "A summary.",
          markdown: "",
          sections: [
            {
              articleId: id,
              article: "Getting started",
              group: "First steps",
              heading: "What it needs",
              anchor: "what-it-needs",
              text: "An encrypted volume, Ollama, and three models.",
            },
          ],
        })),
        Search: vi.fn(async () => []),
        Ask: vi.fn(async () => state.answer),
      },
    },
  };
});

vi.mock("../../bindings/camstuart/talent-hound", () => mocks);

const hit = (articleId: string, article: string, heading: string, snippet: string) => ({
  section: {
    articleId,
    article,
    group: "Rules",
    heading,
    anchor: heading.toLowerCase().replace(/\s+/g, "-"),
    text: `${heading} explained.`,
  },
  score: 1,
  snippet,
});

const search = async (text: string) => {
  fireEvent.input(await screen.findByLabelText("What do you need help with"), { target: { value: text } });
  const button = await screen.findByLabelText("Search the help");
  // The index loads on mount and disables the button while it does.
  await waitFor(() => expect((button as HTMLButtonElement).disabled).toBe(false));
  fireEvent.click(button);
};

beforeEach(() => {
  state.answer = null;
  vi.clearAllMocks();
});

describe("HelpPanel", () => {
  it("lists every topic, grouped, without searching", async () => {
    render(() => <HelpPanel />);
    const index = await screen.findByLabelText("Help topics");
    for (const expected of ["First steps", "Rules", "Tutorial", "Deleting things"]) {
      expect(index.textContent).toContain(expected);
    }
  });

  it("opens an article from the index", async () => {
    render(() => <HelpPanel />);
    fireEvent.click(await screen.findByLabelText("Open Tutorial"));
    await waitFor(() => expect(mocks.HelpService.Article).toHaveBeenCalledWith("tutorial"));
    expect(await screen.findByLabelText("Help article")).toHaveTextContent("Tutorial");
  });

  // Help is read when the rest of the application is the problem.
  it("explains a missing written answer instead of showing an empty one", async () => {
    state.answer = {
      text: "",
      why: "no model is assigned, so there is no written answer.",
      cited: [],
      results: [hit("deleting-things", "Deleting things", "Deleting a candidate", "Initiatives block it.")],
      composed: false,
    };
    render(() => <HelpPanel />);
    await search("delete a candidate");

    expect(await screen.findByLabelText("Why there is no written answer")).toHaveTextContent("no model");
    expect(screen.queryByLabelText("Written answer")).toBeNull();
    expect(await screen.findByLabelText("Search results")).toHaveTextContent("Initiatives block it.");
  });

  it("shows a written answer with the sections it used", async () => {
    const cited = hit("deleting-things", "Deleting things", "Deleting a candidate", "Initiatives block it.");
    state.answer = {
      text: "Delete every initiative that references them first.",
      why: "",
      cited: [cited],
      results: [cited],
      composed: true,
    };
    render(() => <HelpPanel />);
    await search("why can't I delete");

    const answer = await screen.findByLabelText("Written answer");
    expect(answer).toHaveTextContent("Delete every initiative");
    // A model wrote it, and the interface says so.
    expect(answer.getAttribute("data-provenance")).toBe("ai");
    expect(await screen.findByLabelText("Sections this answer used")).toHaveTextContent("Deleting a candidate");
  });

  it("says plainly when nothing matches", async () => {
    state.answer = { text: "", why: "nothing matches that.", cited: [], results: [], composed: false };
    render(() => <HelpPanel />);
    await search("zzz");
    expect(await screen.findByLabelText("Search results")).toHaveTextContent("Nothing in the manual matches");
  });

  it("opens the article a result came from", async () => {
    state.answer = {
      text: "",
      why: "no model.",
      cited: [],
      results: [hit("deleting-things", "Deleting things", "Deleting a candidate", "Initiatives block it.")],
      composed: false,
    };
    render(() => <HelpPanel />);
    await search("delete");
    fireEvent.click(await screen.findByLabelText("Read Deleting things: Deleting a candidate"));
    await waitFor(() => expect(mocks.HelpService.Article).toHaveBeenCalledWith("deleting-things"));
  });
});
