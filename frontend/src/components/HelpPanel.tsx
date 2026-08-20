import { createAction } from "../act";
import { createSignal, For, onMount, Show } from "solid-js";
import { HelpService } from "../../bindings/camstuart/talent-hound";
import type { Answer, Index } from "../../bindings/camstuart/talent-hound";
import type { Article, Hit } from "../../bindings/camstuart/talent-hound/internal/help";

// Help is read when the rest of the application is the problem, so this panel
// asks nothing of it: no initiative, no model, no indexed corpus. Search runs
// against documentation shipped in the binary, and a written answer appears
// only when a model can give one that cites its sections.
export default function HelpPanel() {
  const [index, setIndex] = createSignal<Index[]>([]);
  const [article, setArticle] = createSignal<Article | null>(null);
  const [query, setQuery] = createSignal("");
  const [answer, setAnswer] = createSignal<Answer | null>(null);
  const { act, error, busy } = createAction();

  onMount(() =>
    act(async () => {
      setIndex(((await HelpService.Topics()) ?? []) as Index[]);
      // The tutorial is what a new recruiter needs, so it opens on nothing.
      setArticle((await HelpService.Article("getting-started")) as Article | null);
    }),
  );

  const open = (id: string) =>
    act(async () => {
      setAnswer(null);
      setArticle((await HelpService.Article(id)) as Article | null);
    });

  const ask = () =>
    act(async () => {
      const question = query().trim();
      if (!question) return;
      setAnswer((await HelpService.Ask(question)) as Answer | null);
    });

  const openHit = (hit: Hit) => open(hit.section.articleId);

  return (
    <section class="record-section help" aria-label="Help">
      <h3>Help</h3>
      <p class="muted">
        Everything here is stored in the application. Searching sends nothing anywhere, and works with no
        model installed and no network.
      </p>

      <Show when={error()}>
        <p class="modal-error" role="alert">{error()}</p>
      </Show>

      <form
        class="help-search"
        aria-label="Search help"
        onSubmit={(e) => {
          e.preventDefault();
          void ask();
        }}
      >
        <input
          aria-label="What do you need help with"
          placeholder="What do you need help with?"
          value={query()}
          onInput={(e) => setQuery(e.currentTarget.value)}
        />
        <button class="primary" aria-label="Search the help" disabled={busy()}>
          Search
        </button>
      </form>

      <Show when={answer()}>
        {(a) => (
          <div class="extraction-view" aria-label="Help answer">
            <Show when={a().composed} fallback={<p class="muted" aria-label="Why there is no written answer">{a().why}</p>}>
              {/* Written by a model from the sections below, which is why they
                  are listed with it rather than after it. */}
              <p data-provenance="ai" aria-label="Written answer">
                {a().text}
              </p>
              <ul class="record-list" aria-label="Sections this answer used">
                <For each={a().cited ?? []}>
                  {(hit) => (
                    <li class="search-hit">
                      <button aria-label={`Open ${hit.section.article}: ${hit.section.heading}`} onClick={() => openHit(hit)}>
                        {hit.section.article} — {hit.section.heading}
                      </button>
                    </li>
                  )}
                </For>
              </ul>
            </Show>

            <ul class="record-list" aria-label="Search results">
              <For each={a().results ?? []} fallback={<li class="muted">Nothing in the manual matches that.</li>}>
                {(hit) => (
                  <li class="search-hit">
                    <span class="artifact-name">
                      {hit.section.article} — {hit.section.heading}
                    </span>
                    <p class="muted">{hit.snippet}</p>
                    <button aria-label={`Read ${hit.section.article}: ${hit.section.heading}`} onClick={() => openHit(hit)}>
                      Read this
                    </button>
                  </li>
                )}
              </For>
            </ul>
          </div>
        )}
      </Show>

      <div class="help-body">
        <nav class="help-index" aria-label="Help topics">
          <For each={index()}>
            {(group) => (
              <div>
                <h4>{group.group}</h4>
                <ul class="record-list">
                  <For each={group.topics}>
                    {(topic) => (
                      <li>
                        <button
                          aria-label={`Open ${topic.title}`}
                          data-current={article()?.id === topic.id ? "yes" : "no"}
                          onClick={() => open(topic.id)}
                        >
                          {topic.title}
                        </button>
                        <span class="muted">{topic.summary}</span>
                      </li>
                    )}
                  </For>
                </ul>
              </div>
            )}
          </For>
        </nav>

        <article class="help-article" aria-label="Help article">
          <Show when={article()} fallback={<p class="muted">Choose a topic.</p>}>
            {(a) => (
              <>
                <h4>{a().title}</h4>
                <p class="muted">{a().summary}</p>
                <For each={a().sections}>
                  {(section) => (
                    <section aria-label={`${a().title}: ${section.heading}`}>
                      <h5>{section.heading}</h5>
                      {/* Shipped documentation, displayed as text: this panel
                          renders no markup, here or anywhere else. */}
                      <p class="help-text">{section.text}</p>
                    </section>
                  )}
                </For>
              </>
            )}
          </Show>
        </article>
      </div>
    </section>
  );
}
