import { createAction } from "../act";
import { createSignal, For, onMount, Show } from "solid-js";
import { ChunkService, EmbedService, SearchService } from "../../bindings/camstuart/talent-hound";
import type { Citation, Coverage, Hit, SemanticHit } from "../../bindings/camstuart/talent-hound";

// Search over this workspace's evidence, by word and by meaning. Everything
// shown here is text out of a document a stranger wrote, so it is displayed and
// nothing else: no markup is rendered, nothing is interpreted.
//
// The two modes stay two lists on purpose. Combining them is a weighting
// decision, and there is nothing to judge a weighting against until there is a
// shortlist to be right or wrong about.
type Mode = "words" | "meaning";

export default function SearchPanel(props: { initiativeId: number }) {
  const [query, setQuery] = createSignal("");
  const [mode, setMode] = createSignal<Mode>("words");
  const [hits, setHits] = createSignal<Hit[]>([]);
  const [semantic, setSemantic] = createSignal<SemanticHit[]>([]);
  const [searched, setSearched] = createSignal(false);
  const [chunks, setChunks] = createSignal(0);
  const [coverage, setCoverage] = createSignal<Coverage | null>(null);
  const [citation, setCitation] = createSignal<Citation | null>(null);
  // The backend's own words, verbatim: it knows rules the UI does not.
  const { act, error } = createAction();

  const count = () =>
    act(async () => {
      setChunks(await ChunkService.CountForInitiative(props.initiativeId));
      setCoverage((await EmbedService.Coverage(props.initiativeId)) as Coverage);
    });
  onMount(() => void count());

  const search = () =>
    act(async () => {
      setCitation(null);
      setSearched(true);
      if (mode() === "meaning") {
        setHits([]);
        setSemantic(((await EmbedService.SemanticSearch(props.initiativeId, query(), 0)) ?? []) as SemanticHit[]);
        return;
      }
      setSemantic([]);
      setHits(((await SearchService.Search(props.initiativeId, query(), 0)) ?? []) as Hit[]);
    });

  const index = () =>
    act(async () => {
      await ChunkService.ChunkAll(props.initiativeId);
      // The job runs in the background; the count catches up when it lands.
      setTimeout(() => void count(), 500);
    });

  const embed = () =>
    act(async () => {
      // Chunks for reading and citing, aspects for matching: the shortlist
      // compares a candidate's statements against a role's, not against the
      // blurb around them.
      await EmbedService.EmbedAll(props.initiativeId);
      await EmbedService.EmbedAspects(props.initiativeId);
      setTimeout(() => void count(), 500);
    });

  const cite = (chunkId: number) => act(async () => setCitation((await SearchService.Cite(chunkId)) as Citation));

  // A chunk in a document with no headings has an empty path, and the backend
  // may send it as null rather than an empty list.
  const path = (headings: string[] | null) => (headings && headings.length > 0 ? headings.join(" › ") : "no section");

  // Coverage is the honest answer to "why did that find nothing": a model
  // change strands the corpus, and the number says so rather than the results
  // quietly narrowing.
  const embedded = () => {
    const c = coverage();
    if (!c) return "";
    if (!c.space) return `no embedding model has indexed this initiative — ${c.total} sections outstanding`;
    return `${c.embedded} of ${c.total} sections embedded with ${c.space.model} (revision ${c.space.revision}, ${c.space.dimensions} dimensions)`;
  };

  return (
    <section class="record-section" aria-label="Search">
      <h3>Search</h3>
      <p class="muted">
        {chunks()} indexed {chunks() === 1 ? "section" : "sections"} of this initiative's extracted artifacts.
      </p>
      <p class="muted" aria-label="Embedding coverage">
        {embedded()}
      </p>

      <div class="search-bar">
        <select aria-label="Search by" value={mode()} onChange={(e) => setMode(e.currentTarget.value as Mode)}>
          <option value="words">Words</option>
          <option value="meaning">Meaning</option>
        </select>
        <input
          aria-label="Search evidence"
          placeholder={mode() === "meaning" ? "Describe what you are looking for" : "Words to look for"}
          value={query()}
          onInput={(e) => setQuery(e.currentTarget.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") search();
          }}
        />
        <button class="primary" onClick={search}>
          Search
        </button>
        <button aria-label="Index this initiative's artifacts" onClick={index}>
          Index artifacts
        </button>
        <button aria-label="Embed this initiative's sections" onClick={embed}>
          Embed sections
        </button>
      </div>

      <Show when={error()}>
        <p class="modal-error">{error()}</p>
      </Show>

      <ul class="record-list" aria-label="Search results">
        <For
          each={hits()}
          fallback={
            <Show when={searched() && mode() === "words"}>
              <li class="muted">Nothing matched every word.</li>
            </Show>
          }
        >
          {(hit) => (
            <li class="search-hit">
              <span class="artifact-name">
                {hit.artifactName}
                <span class="muted"> — {path(hit.headingPath)}</span>
              </span>
              <button aria-label={`Cite ${hit.artifactName} section ${hit.ordinal + 1}`} onClick={() => cite(hit.chunkId)}>
                Cite
              </button>
              {/* Untrusted: displayed, never rendered, never acted on. */}
              <pre aria-label={`Match in ${hit.artifactName} section ${hit.ordinal + 1}`}>{hit.text}</pre>
            </li>
          )}
        </For>
        <For
          each={semantic()}
          fallback={
            <Show when={searched() && mode() === "meaning"}>
              <li class="muted">Nothing is embedded to compare against.</li>
            </Show>
          }
        >
          {(hit) => (
            <li class="search-hit">
              <span class="artifact-name">
                {hit.artifactName}
                <span class="muted">
                  {" "}
                  — {path(hit.headingPath)} — similarity {hit.score.toFixed(3)}
                </span>
              </span>
              <button
                aria-label={`Cite ${hit.artifactName} section ${hit.ordinal + 1}`}
                onClick={() => cite(hit.chunkId)}
              >
                Cite
              </button>
              {/* Untrusted: displayed, never rendered, never acted on. */}
              <pre aria-label={`Match in ${hit.artifactName} section ${hit.ordinal + 1}`}>{hit.text}</pre>
            </li>
          )}
        </For>
      </ul>

      <Show when={citation()}>
        {(c) => (
          <div class="extraction-view">
            <h4>
              {c().location}
              <button aria-label="Close the citation" onClick={() => setCitation(null)}>
                Close
              </button>
            </h4>
            <p class="muted">
              Characters {c().startOffset}–{c().endOffset} of {c().filename || c().artifact}
            </p>
            <pre aria-label="Cited text">{c().text}</pre>
          </div>
        )}
      </Show>
    </section>
  );
}
