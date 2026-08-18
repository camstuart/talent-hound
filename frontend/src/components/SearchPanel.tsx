import { createSignal, For, onMount, Show } from "solid-js";
import { ChunkService, SearchService } from "../../bindings/camstuart/talent-hound";
import type { Citation, Hit } from "../../bindings/camstuart/talent-hound";

// Lexical search over this workspace's evidence. Everything shown here is text
// out of a document a stranger wrote, so it is displayed and nothing else: no
// markup is rendered, nothing is interpreted.
export default function SearchPanel(props: { initiativeId: number }) {
  const [query, setQuery] = createSignal("");
  const [hits, setHits] = createSignal<Hit[]>([]);
  const [searched, setSearched] = createSignal(false);
  const [chunks, setChunks] = createSignal(0);
  const [citation, setCitation] = createSignal<Citation | null>(null);
  const [error, setError] = createSignal("");

  // Every action reports the backend's own words: it knows rules the UI does not.
  const act = async (run: () => Promise<unknown>) => {
    setError("");
    try {
      await run();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    }
  };

  const count = () => act(async () => setChunks(await ChunkService.CountForInitiative(props.initiativeId)));
  onMount(() => void count());

  const search = () =>
    act(async () => {
      setCitation(null);
      setHits(((await SearchService.Search(props.initiativeId, query(), 0)) ?? []) as Hit[]);
      setSearched(true);
    });

  const index = () =>
    act(async () => {
      await ChunkService.ChunkAll(props.initiativeId);
      // The job runs in the background; the count catches up when it lands.
      setTimeout(() => void count(), 500);
    });

  const cite = (hit: Hit) => act(async () => setCitation((await SearchService.Cite(hit.chunkId)) as Citation));

  // A chunk in a document with no headings has an empty path, and the backend
  // may send it as null rather than an empty list.
  const path = (headings: string[] | null) => (headings && headings.length > 0 ? headings.join(" › ") : "no section");

  return (
    <section class="record-section" aria-label="Search">
      <h3>Search</h3>
      <p class="muted">
        {chunks()} indexed {chunks() === 1 ? "section" : "sections"} of this initiative's extracted artifacts.
      </p>

      <div class="search-bar">
        <input
          aria-label="Search evidence"
          placeholder="Words to look for"
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
      </div>

      <Show when={error()}>
        <p class="modal-error">{error()}</p>
      </Show>

      <ul class="record-list" aria-label="Search results">
        <For
          each={hits()}
          fallback={
            <Show when={searched()}>
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
              <button aria-label={`Cite ${hit.artifactName} section ${hit.ordinal + 1}`} onClick={() => cite(hit)}>
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
