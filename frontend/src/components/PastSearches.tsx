import { For } from "solid-js";
import type { Search } from "../../bindings/camstuart/talent-hound/internal/models";

// Every search that was sent, with its exact query — kept so a shortlist or a
// lead can be traced to the text that produced it, and a refused attempt is
// shown as itself rather than as a search that never happened.
export default function PastSearches(props: {
  label: string;
  emptyText: string;
  queryLabel: (index: number) => string;
  searches: Search[];
}) {
  return (
    <ul class="record-list" aria-label={props.label}>
      <For each={props.searches} fallback={<li class="muted">{props.emptyText}</li>}>
        {(s, i) => (
          <li class="search-hit">
            <span class="artifact-name">
              {s.provider}
              <span class="muted">
                {" "}
                — {s.failureReason ? s.failureReason.replace(/_/g, " ") : `${s.resultCount} results`}
                {s.partial ? ", partial" : ""}
              </span>
            </span>
            <pre aria-label={props.queryLabel(i() + 1)}>{s.query}</pre>
          </li>
        )}
      </For>
    </ul>
  );
}
