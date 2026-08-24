import { createEffect, createSignal, For, Show } from "solid-js";
import { RecordService, SearchService } from "../../bindings/camstuart/talent-hound";
import type { PersonHit } from "../../bindings/camstuart/talent-hound";
import { createAction } from "../act";

// The recruiter's whole pool, cross-initiative. Two searches on purpose: the
// filter answers "who matches these facts", the talent search answers "whose
// evidence talks about this" — merging them would leave both unexplainable.

export type CrmKind = "candidate" | "company" | "contact" | "role";
const KINDS: { kind: CrmKind; label: string }[] = [
  { kind: "candidate", label: "Candidates" },
  { kind: "company", label: "Companies" },
  { kind: "contact", label: "Contacts" },
  { kind: "role", label: "Roles" },
];

type Row = { id: number; title: string; subtitle: string };

const rows = async (kind: CrmKind, text: string): Promise<Row[]> => {
  switch (kind) {
    case "candidate": {
      const cs =
        (await RecordService.SearchCandidates({
          text,
          workRights: "",
          employmentType: "",
          arrangement: "",
          availableBy: "",
        })) ?? [];
      return cs.map((c) => ({ id: c.id, title: c.fullName, subtitle: c.location ?? "" }));
    }
    case "company": {
      const cs = (await RecordService.SearchCompanies(text)) ?? [];
      return cs.map((c) => ({ id: c.id, title: c.name, subtitle: "" }));
    }
    case "contact": {
      const cs = (await RecordService.SearchContacts(text)) ?? [];
      return cs.map((c) => ({ id: c.id, title: c.fullName, subtitle: c.email ?? "" }));
    }
    case "role": {
      const rs = (await RecordService.ListRoles()) ?? [];
      const t = text.trim().toLowerCase();
      return rs
        .filter((r) => !t || r.title.toLowerCase().includes(t))
        .map((r) => ({ id: r.id, title: r.title, subtitle: "" }));
    }
  }
};

export default function CrmPanel() {
  const [kind, setKind] = createSignal<CrmKind>("candidate");
  const [filter, setFilter] = createSignal("");
  const [applied, setApplied] = createSignal("");
  const [selected, setSelected] = createSignal<{ type: CrmKind; id: number } | null>(null);
  const [people, setPeople] = createSignal<PersonHit[] | null>(null);
  const [talentQuery, setTalentQuery] = createSignal("");
  const [list, setList] = createSignal<Row[]>([]);
  // The backend's own words, verbatim: it knows rules the UI does not — a
  // rejected search must land in error() and never throw through render.
  const { act, reloader, error } = createAction();

  const reload = reloader(async (isCurrent) => {
    const found = await rows(kind(), applied());
    if (!isCurrent()) return;
    setList(found);
  });

  createEffect(() => {
    kind();
    applied();
    void reload();
  });

  const runTalent = (e: Event) => {
    e.preventDefault();
    void act(async () => {
      setPeople(((await SearchService.People(talentQuery(), 20)) ?? []) as PersonHit[]);
    });
  };

  return (
    <div class="container crm" aria-label="CRM">
      <aside class="crm-list">
        <div class="area-tabs" role="tablist" aria-label="Record types">
          <For each={KINDS}>
            {(k) => (
              <button
                class="area-tab"
                classList={{ active: kind() === k.kind }}
                role="tab"
                aria-selected={kind() === k.kind}
                onClick={() => {
                  setKind(k.kind);
                  setSelected(null);
                  setPeople(null);
                }}
              >
                {k.label}
              </button>
            )}
          </For>
        </div>

        <form
          aria-label="Filter form"
          onSubmit={(e) => {
            e.preventDefault();
            setApplied(filter());
          }}
        >
          <input
            aria-label="Filter"
            placeholder="Filter by name, email, location…"
            value={filter()}
            onInput={(e) => setFilter(e.currentTarget.value)}
          />
        </form>

        <Show when={kind() === "candidate"}>
          <form aria-label="Talent search form" onSubmit={runTalent}>
            <input
              aria-label="Talent search"
              placeholder="Search the talent pool's evidence…"
              value={talentQuery()}
              onInput={(e) => setTalentQuery(e.currentTarget.value)}
            />
          </form>
        </Show>

        <Show when={error()}>
          <p class="modal-error" role="alert">{error()}</p>
        </Show>

        <Show
          when={people()}
          fallback={
            <ul class="record-list" aria-label="Records">
              <For each={list()}>
                {(r) => (
                  <li
                    class="search-hit"
                    classList={{ active: selected()?.id === r.id }}
                    onClick={() => setSelected({ type: kind(), id: r.id })}
                  >
                    <span class="artifact-name">{r.title}</span>
                    <span class="muted">{r.subtitle}</span>
                  </li>
                )}
              </For>
            </ul>
          }
        >
          {(hits) => (
            <ul class="record-list" aria-label="Talent search results">
              <For each={hits()}>
                {(h) => (
                  <li class="search-hit" onClick={() => setSelected({ type: "candidate", id: h.candidate.id })}>
                    <span class="artifact-name">{h.candidate.fullName}</span>
                    <span class="muted">{h.artifactName}</span>
                    <span class="shell-note">{h.snippet}</span>
                  </li>
                )}
              </For>
              <button class="muted" onClick={() => setPeople(null)}>
                Back to the list
              </button>
            </ul>
          )}
        </Show>
      </aside>

      <section class="crm-detail" aria-label="Record detail">
        <Show when={selected()} fallback={<p class="muted">Select a record to see its details and history.</p>}>
          {(sel) => <p class="muted">{/* Task 7 replaces this */}Selected {sel().type} #{sel().id}</p>}
        </Show>
      </section>
    </div>
  );
}
