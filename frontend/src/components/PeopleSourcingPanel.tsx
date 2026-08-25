import { createAction } from "../act";
import { createEffect, createSignal, For, Show } from "solid-js";
import { Browser } from "@wailsio/runtime";
import { RecordService, SourcingService } from "../../bindings/camstuart/talent-hound";
import type { LeadView, QueryPreview, SourcingOutcome } from "../../bindings/camstuart/talent-hound";
import type { Candidate, Role, Search } from "../../bindings/camstuart/talent-hound/internal/models";
import RecordForm from "./RecordForm";
import { bumpWorkspace, workspaceRevision } from "../workspaceRevision";

// The mirror of role discovery: a role's requirements go out, people come
// back. The same rules hold — the recruiter sees the exact query, nothing is
// sent until they confirm — and one more: nothing that comes back is a
// candidate until the recruiter promotes it. A result is a page, and pages are
// not people the pool has agreed to hold.
export default function PeopleSourcingPanel(props: { initiativeId: number }) {
  const [roles, setRoles] = createSignal<Role[]>([]);
  const [role, setRole] = createSignal(0);
  const [preview, setPreview] = createSignal<QueryPreview | null>(null);
  const [query, setQuery] = createSignal("");
  const [outcome, setOutcome] = createSignal<SourcingOutcome | null>(null);
  const [leads, setLeads] = createSignal<LeadView[]>([]);
  const [searches, setSearches] = createSignal<Search[]>([]);
  const [promoting, setPromoting] = createSignal<LeadView | null>(null);
  const [suggested, setSuggested] = createSignal<Record<string, string>>({});
  // The backend's own words, verbatim: it knows rules the UI does not.
  const { act, reloader, error, busy, setError } = createAction();

  const reload = reloader(async (isCurrent) => {
    const [roles, leads, searches] = await Promise.all([
      RecordService.ListRoles(),
      SourcingService.Leads(props.initiativeId, ""),
      SourcingService.Searches(props.initiativeId),
    ]);
    if (!isCurrent()) return;
    setRoles((roles ?? []) as Role[]);
    setLeads((leads ?? []) as LeadView[]);
    setSearches((searches ?? []) as Search[]);
  });

  createEffect(() => {
    workspaceRevision();
    void reload();
  });

  const build = () =>
    act(async () => {
      setOutcome(null);
      const p = (await SourcingService.Preview(role())) as QueryPreview;
      setPreview(p);
      setQuery(p.query);
    });

  // Editing re-inspects: the warnings are about what is in the box now.
  const edit = (text: string) =>
    act(async () => {
      setQuery(text);
      setPreview(((await SourcingService.Inspect(role(), text)) ?? null) as QueryPreview | null);
    });

  // Cancelling is the absence of the operation: it calls nothing at all.
  const cancel = () => {
    setPreview(null);
    setQuery("");
    setError("");
  };

  const send = () =>
    act(async () => {
      try {
        const result = (await SourcingService.Send({
          initiativeId: props.initiativeId,
          roleId: role(),
          query: query(),
          limit: 20,
        } as never)) as SourcingOutcome;
        setOutcome(result);
        setPreview(null);
      } finally {
        // A refused or failed request is still an attempt with a recorded
        // reason, so the lists refresh either way.
        await reload();
      }
    });

  const dismiss = (id: number) =>
    act(async () => {
      await SourcingService.Dismiss(id);
      await reload();
    });

  // The page opens in the recruiter's own browser. It is never rendered here:
  // a stranger's page inside this window would be a stranger's page with this
  // application's permissions.
  const open = (url: string) => act(() => Browser.OpenURL(url));

  const startPromote = (lead: LeadView) =>
    act(async () => {
      const guess = (await SourcingService.Suggest(lead.id)) as Candidate;
      setSuggested({
        fullName: guess?.fullName ?? "",
        preferredName: "",
        location: guess?.location ?? "",
        sourceNote: guess?.sourceNote ?? "",
      });
      setPromoting(lead);
    });

  const promote = async (values: Record<string, string>) => {
    const lead = promoting();
    if (!lead) return;
    await SourcingService.Promote(lead.id, {
      fullName: values.fullName,
      preferredName: values.preferredName,
      location: values.location,
      sourceNote: values.sourceNote,
    } as never);
    setPromoting(null);
    bumpWorkspace();
    await reload();
  };

  const stateLabel = (lead: LeadView) => {
    if (lead.state === "promoted") return "promoted";
    if (lead.state === "dismissed") return "dismissed";
    if (lead.candidateName) return `in pool as ${lead.candidateName}`;
    return "";
  };

  return (
    <section class="record-section" aria-label="Find people">
      <h3>Find people</h3>
      <p class="muted">
        Queries are built from a role's profile with the client and its contacts removed. Nothing is sent until you
        have seen the exact text and confirmed it, and nobody becomes a candidate until you promote them.
      </p>

      <div class="search-bar">
        <select
          aria-label="Search for role"
          value={String(role())}
          onFocus={() => void reload()}
          onChange={(e) => setRole(Number(e.currentTarget.value))}
        >
          <option value="0">Choose a role</option>
          <For each={roles()}>{(r) => <option value={String(r.id)}>{r.title}</option>}</For>
        </select>
        <button aria-label="Build a people query" disabled={busy() || role() === 0} onClick={build}>
          Build a query
        </button>
      </div>

      <Show when={error()}>
        <p class="modal-error" role="alert">{error()}</p>
      </Show>

      <Show when={preview()}>
        {(p) => (
          <div class="extraction-view" role="region" aria-label="People query preview">
            <h4>This is exactly what will be sent</h4>
            <textarea
              aria-label="People query to send"
              rows="3"
              value={query()}
              onInput={(e) => edit(e.currentTarget.value)}
            />
            <Show when={p().organizationWarning}>
              <p class="shell-note" aria-label="Organization warning">{p().organizationWarning}</p>
            </Show>
            <Show when={p().identifierWarning}>
              <p class="modal-error" aria-label="Identifier warning">{p().identifierWarning}</p>
            </Show>
            <button class="primary" aria-label="Send this people search" disabled={busy()} onClick={send}>
              Send it
            </button>
            <button aria-label="Cancel this people search" onClick={cancel}>
              Cancel
            </button>
          </div>
        )}
      </Show>

      <Show when={outcome()}>
        {(o) => (
          <p class="muted" aria-label="People search outcome">
            {o().created} {o().created === 1 ? "lead" : "leads"}
            <Show when={o().alreadyInPool}> — {o().alreadyInPool} already in the pool</Show>
            <Show when={o().partial}>
              {" "}— {o().skipped} {o().skipped === 1 ? "result" : "results"} could not be read, so this is incomplete
            </Show>
          </p>
        )}
      </Show>

      <Show when={promoting()}>
        {(lead) => (
          <div class="extraction-view" role="region" aria-label="Promote lead">
            <p class="muted">
              Promoting <span class="artifact-name">{lead().title || lead().url}</span>. Correct the details; the page's
              text is kept as evidence, not as fields.
            </p>
            <RecordForm
              legend="Promote to candidate"
              fields={[
                { key: "fullName", label: "Full name", required: true, match: "full name" },
                { key: "preferredName", label: "Preferred name" },
                { key: "location", label: "Location" },
                { key: "sourceNote", label: "Source or authority" },
              ]}
              submitLabel="Promote"
              initial={suggested()}
              onSubmit={promote}
            />
            <button aria-label="Cancel promotion" onClick={() => setPromoting(null)}>
              Cancel
            </button>
          </div>
        )}
      </Show>

      <ul class="record-list" aria-label="Leads">
        <For each={leads()} fallback={<li class="muted">No leads yet.</li>}>
          {(lead) => (
            <li class="search-hit" aria-label={`Lead ${lead.title || lead.url}`}>
              <span class="artifact-name">
                {lead.title || lead.url}
                <span class="muted"> — {lead.host}</span>
                <Show when={stateLabel(lead)}>
                  <span class="muted"> ({stateLabel(lead)})</span>
                </Show>
              </span>
              {/* A stranger's page, as the provider quoted it: displayed, never rendered. */}
              <Show when={lead.snippet}>
                <pre>{lead.snippet}</pre>
              </Show>
              <div class="search-bar">
                <button aria-label={`Open ${lead.title || lead.url}`} onClick={() => open(lead.url)}>
                  Open
                </button>
                <Show when={lead.state === "new" && !lead.candidateId}>
                  <button
                    class="primary"
                    aria-label={`Promote ${lead.title || lead.url}`}
                    disabled={busy()}
                    onClick={() => startPromote(lead)}
                  >
                    Promote
                  </button>
                </Show>
                <Show when={lead.state === "new"}>
                  <button aria-label={`Dismiss ${lead.title || lead.url}`} disabled={busy()} onClick={() => dismiss(lead.id)}>
                    Dismiss
                  </button>
                </Show>
              </div>
            </li>
          )}
        </For>
      </ul>

      <ul class="record-list" aria-label="Past people searches">
        <For each={searches()} fallback={<li class="muted">No people searches yet.</li>}>
          {(s, i) => (
            <li class="search-hit">
              <span class="artifact-name">
                {s.provider}
                <span class="muted">
                  {" "}— {s.failureReason ? s.failureReason.replace(/_/g, " ") : `${s.resultCount} results`}
                  {s.partial ? ", partial" : ""}
                </span>
              </span>
              <pre aria-label={`People query sent for search ${i() + 1}`}>{s.query}</pre>
            </li>
          )}
        </For>
      </ul>
    </section>
  );
}
