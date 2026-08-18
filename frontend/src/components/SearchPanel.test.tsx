import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@solidjs/testing-library";
import SearchPanel from "./SearchPanel";

// The Go backend is not running: bindings are mocked. Fixtures are invented.
const { state, searchMocks, chunkMocks, embedMocks } = vi.hoisted(() => {
  const state = {
    hits: [] as Record<string, unknown>[],
    semantic: [] as Record<string, unknown>[],
    citation: null as Record<string, unknown> | null,
    count: 0,
    searchError: "",
    semanticError: "",
    coverage: null as Record<string, unknown> | null,
  };
  return {
    state,
    embedMocks: {
      Coverage: vi.fn(async () => state.coverage ?? { space: null, total: 0, embedded: 0, outstanding: 0 }),
      SemanticSearch: vi.fn(async () => {
        if (state.semanticError) throw new Error(state.semanticError);
        return state.semantic;
      }),
      EmbedAll: vi.fn(async () => ({ id: 11, kind: "embed", state: "queued", totalItems: 3 })),
      CurrentSpace: vi.fn(async () => state.coverage?.space ?? null),
      Spaces: vi.fn(async () => []),
    },
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
  EmbedService: embedMocks,
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

const aSemanticHit = (over: Record<string, unknown> = {}) => ({
  ...aHit(),
  score: 0.8421,
  spaceId: 1,
  ...over,
});

beforeEach(() => {
  state.hits = [];
  state.semantic = [];
  state.citation = null;
  state.count = 0;
  state.searchError = "";
  state.semanticError = "";
  state.coverage = null;
  vi.clearAllMocks();
});

const searchFor = async (query: string) => {
  fireEvent.input(screen.getByLabelText("Search evidence"), { target: { value: query } });
  fireEvent.click(screen.getByRole("button", { name: "Search" }));
};

const searchByMeaning = async (query: string) => {
  fireEvent.change(screen.getByLabelText("Search by"), { target: { value: "meaning" } });
  await searchFor(query);
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

  it("searches by meaning and shows the similarity of each hit", async () => {
    state.semantic = [aSemanticHit()];
    render(() => <SearchPanel initiativeId={4} />);
    await searchByMeaning("someone who has run infrastructure");

    await screen.findByText(/similarity 0\.842/);
    expect(embedMocks.SemanticSearch).toHaveBeenCalledWith(4, "someone who has run infrastructure", 0);
    // The two modes are two lists; asking for meaning does not also run words.
    expect(searchMocks.Search).not.toHaveBeenCalled();
  });

  it("says which model indexed the workspace and how much of it", async () => {
    state.coverage = {
      space: { id: 1, model: "nomic-embed-text", revision: 2, dimensions: 768 },
      total: 40,
      embedded: 40,
      outstanding: 0,
    };
    render(() => <SearchPanel initiativeId={1} />);
    await screen.findByText(/40 of 40 sections embedded with nomic-embed-text \(revision 2, 768 dimensions\)/);
  });

  it("says what is outstanding when nothing has been embedded", async () => {
    state.coverage = { space: null, total: 12, embedded: 0, outstanding: 12 };
    render(() => <SearchPanel initiativeId={1} />);
    await screen.findByText(/no embedding model has indexed this initiative — 12 sections outstanding/);
  });

  it("embeds the workspace's sections on request", async () => {
    render(() => <SearchPanel initiativeId={5} />);
    fireEvent.click(await screen.findByLabelText("Embed this initiative's sections"));
    await waitFor(() => expect(embedMocks.EmbedAll).toHaveBeenCalledWith(5));
  });

  it("shows the backend's own words when a semantic search cannot run", async () => {
    state.semanticError = "nothing is embedded yet for the current model — index this initiative first";
    render(() => <SearchPanel initiativeId={1} />);
    await searchByMeaning("anything");
    await screen.findByText("nothing is embedded yet for the current model — index this initiative first");
  });
});
