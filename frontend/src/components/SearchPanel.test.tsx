import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@solidjs/testing-library";
import SearchPanel from "./SearchPanel";

// The Go backend is not running: bindings are mocked. Fixtures are invented.
const { state, searchMocks, chunkMocks } = vi.hoisted(() => {
  const state = {
    hits: [] as Record<string, unknown>[],
    citation: null as Record<string, unknown> | null,
    count: 0,
    searchError: "",
  };
  return {
    state,
    searchMocks: {
      Search: vi.fn(async () => {
        if (state.searchError) throw new Error(state.searchError);
        return state.hits;
      }),
      Cite: vi.fn(async () => state.citation),
      Rebuild: vi.fn(async () => undefined),
    },
    chunkMocks: {
      CountForInitiative: vi.fn(async () => state.count),
      ChunkAll: vi.fn(async () => ({ id: 9, kind: "chunk", state: "queued", totalItems: 2 })),
      Chunk: vi.fn(async () => ({ id: 10 })),
      List: vi.fn(async () => []),
    },
  };
});

vi.mock("../../bindings/camstuart/talent-hound", () => ({
  SearchService: searchMocks,
  ChunkService: chunkMocks,
}));

const aHit = (over: Record<string, unknown> = {}) => ({
  chunkId: 3,
  artifactId: 1,
  artifactName: "Role brief",
  ordinal: 1,
  headingPath: ["Platform engineer", "Requirements"],
  text: "Five years building distributed systems in Go.",
  ...over,
});

beforeEach(() => {
  state.hits = [];
  state.citation = null;
  state.count = 0;
  state.searchError = "";
  vi.clearAllMocks();
});

const searchFor = async (query: string) => {
  fireEvent.input(screen.getByLabelText("Search evidence"), { target: { value: query } });
  fireEvent.click(screen.getByRole("button", { name: "Search" }));
};

describe("SearchPanel", () => {
  it("shows how much of the workspace is indexed", async () => {
    state.count = 12;
    render(() => <SearchPanel initiativeId={1} />);
    await screen.findByText(/12 indexed sections/);
  });

  it("searches and shows each hit with its artifact and heading path", async () => {
    state.hits = [aHit()];
    render(() => <SearchPanel initiativeId={1} />);
    await searchFor("distributed systems");

    await screen.findByText("Role brief");
    expect(await screen.findByText(/Platform engineer › Requirements/)).toBeTruthy();
    expect(searchMocks.Search).toHaveBeenCalledWith(1, "distributed systems", 0);
  });

  it("says so when nothing matched", async () => {
    render(() => <SearchPanel initiativeId={1} />);
    await searchFor("kangaroo");
    await screen.findByText("Nothing matched every word.");
  });

  it("shows a match literally, never as markup", async () => {
    // A document a stranger wrote. Nothing here interprets it.
    state.hits = [aHit({ text: "<script>alert('x')</script> **not bold**" })];
    render(() => <SearchPanel initiativeId={1} />);
    await searchFor("script");

    const match = await screen.findByLabelText("Match in Role brief section 2");
    expect(match.tagName).toBe("PRE");
    expect(match.querySelector("script")).toBeNull();
    expect(match.textContent).toBe("<script>alert('x')</script> **not bold**");
  });

  it("resolves a hit to a citation with its location and text", async () => {
    state.hits = [aHit()];
    state.citation = {
      chunkId: 3,
      artifactId: 1,
      artifact: "Role brief",
      filename: "brief.md",
      ordinal: 1,
      headingPath: ["Platform engineer", "Requirements"],
      startOffset: 42,
      endOffset: 110,
      text: "Five years building distributed systems in Go.",
      location: "Role brief — Platform engineer › Requirements (section 2)",
    };
    render(() => <SearchPanel initiativeId={1} />);
    await searchFor("systems");

    fireEvent.click(await screen.findByLabelText("Cite Role brief section 2"));
    await screen.findByText("Role brief — Platform engineer › Requirements (section 2)");
    expect(screen.getByText(/Characters 42–110 of brief.md/)).toBeTruthy();
    const cited = screen.getByLabelText("Cited text");
    expect(cited.tagName).toBe("PRE");
    expect(searchMocks.Cite).toHaveBeenCalledWith(3);
  });

  it("indexes the workspace's artifacts on request", async () => {
    render(() => <SearchPanel initiativeId={7} />);
    fireEvent.click(await screen.findByLabelText("Index this initiative's artifacts"));
    await waitFor(() => expect(chunkMocks.ChunkAll).toHaveBeenCalledWith(7));
  });

  it("shows the backend's own words when a search fails", async () => {
    state.searchError = "the search index is being rebuilt";
    render(() => <SearchPanel initiativeId={1} />);
    await searchFor("anything");
    await screen.findByText("the search index is being rebuilt");
  });
});
