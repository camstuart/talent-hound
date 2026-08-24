import { createEffect, createSignal, For, onMount, Show } from "solid-js";
import {
  CandidateProfileService,
  InitiativeService,
  InteractionService,
  RecordService,
  SearchService,
} from "../../bindings/camstuart/talent-hound";
import type { PersonHit, TimelineEntry } from "../../bindings/camstuart/talent-hound";
import { LinkTarget } from "../../bindings/camstuart/talent-hound/internal/models";
import type { Candidate, Company, Contact, Initiative, Profile, Role } from "../../bindings/camstuart/talent-hound/internal/models";
import { createAction } from "../act";
import RecordForm, { list, num, type FieldSpec } from "./RecordForm";
import ArtifactsPanel from "./ArtifactsPanel";

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

const PERIODS = [
  { value: "", label: "—" },
  { value: "hour", label: "per hour" },
  { value: "day", label: "per day" },
  { value: "week", label: "per week" },
  { value: "month", label: "per month" },
  { value: "year", label: "per year" },
];

// Compensation is four fields wherever it appears — copied from RecordsPanel
// so a candidate or role edited here matches the fields it was created with.
const compensationFields = (): FieldSpec[] => [
  { key: "compMin", label: "Compensation minimum", type: "number", match: "minimum" },
  { key: "compMax", label: "Compensation maximum", type: "number", match: "maximum" },
  { key: "compCurrency", label: "Currency", placeholder: "NZD", match: "currency" },
  { key: "compPeriod", label: "Period", type: "select", options: PERIODS, match: "period" },
];

const compensation = (v: Record<string, string>) => ({
  min: num(v.compMin),
  max: num(v.compMax),
  currency: v.compCurrency,
  period: v.compPeriod,
});

const joinList = (values: string[] | null | undefined) => (values ?? []).join(", ");

const INTERACTION_KINDS = [
  { value: "call", label: "Call" },
  { value: "meeting", label: "Meeting" },
  { value: "email", label: "Email" },
  { value: "note", label: "Note" },
  { value: "placement", label: "Placement" },
  { value: "application", label: "Application" },
  { value: "rejection", label: "Rejection" },
];
// These three kinds mean the interaction settled an outcome, so the form only
// asks which role it was about when it could matter.
const OUTCOME_KINDS = new Set(["placement", "application", "rejection"]);

// The details-form field specs, copied from RecordsPanel.tsx so a record
// created there and edited here share exactly the same fields.
const detailFields = (kind: CrmKind): FieldSpec[] => {
  switch (kind) {
    case "candidate":
      return [
        { key: "fullName", label: "Full name", required: true, match: "full name" },
        { key: "preferredName", label: "Preferred name" },
        { key: "emails", label: "Email addresses", placeholder: "comma separated" },
        { key: "phones", label: "Phone numbers", placeholder: "comma separated" },
        { key: "location", label: "Location" },
        { key: "workRights", label: "Work rights or visa" },
        { key: "availability", label: "Available from", type: "date", match: "availability" },
        { key: "desiredEmploymentType", label: "Desired employment type" },
        { key: "desiredWorkArrangement", label: "Desired work arrangement" },
        ...compensationFields(),
        { key: "sourceNote", label: "Source or authority" },
        { key: "lastConfirmed", label: "Last confirmed", type: "date", match: "last-confirmed" },
      ];
    case "company":
      return [
        { key: "name", label: "Name", required: true, match: "company name" },
        { key: "website", label: "Website", placeholder: "https://example.test", match: "website" },
        { key: "location", label: "Location" },
        { key: "source", label: "Source" },
      ];
    case "contact":
      return [
        { key: "companyId", label: "Company", type: "select", options: [], match: "company" },
        { key: "fullName", label: "Full name", required: true, match: "contact full name" },
        { key: "title", label: "Role or title" },
        { key: "email", label: "Email" },
        { key: "phone", label: "Phone" },
        { key: "source", label: "Source" },
      ];
    case "role":
      return [
        { key: "title", label: "Title", required: true, match: "role title" },
        { key: "companyName", label: "Company name" },
        { key: "companyId", label: "Company record", type: "select", options: [] },
        { key: "location", label: "Location" },
        { key: "workArrangement", label: "Work arrangement" },
        { key: "employmentType", label: "Employment type" },
        ...compensationFields(),
        { key: "publishedOn", label: "Published", type: "date", match: "published" },
        { key: "closingOn", label: "Closing", type: "date", match: "closing" },
        { key: "retrievedOn", label: "Retrieved", type: "date", match: "retrieved" },
        { key: "sourceId", label: "Source ID" },
        { key: "canonicalUrl", label: "Canonical URL", match: "canonical url" },
        { key: "source", label: "Source" },
        {
          key: "origin",
          label: "Origin",
          type: "select",
          match: "origin",
          options: [
            { value: "recruiter_entered", label: "Recruiter entered" },
            { value: "discovered", label: "Discovered" },
          ],
        },
        {
          key: "lifecycleState",
          label: "Lifecycle state",
          type: "select",
          match: "lifecycle state",
          options: [
            { value: "open", label: "Open (recruiter entered)" },
            { value: "filled", label: "Filled (recruiter entered)" },
            { value: "closed", label: "Closed (recruiter entered)" },
            { value: "active", label: "Active (discovered)" },
            { value: "stale", label: "Stale (discovered)" },
            { value: "purged", label: "Purged (discovered)" },
          ],
        },
      ];
  }
};

const CREATE_LABELS: Record<CrmKind, string> = {
  candidate: "Add candidate",
  company: "Add company",
  contact: "Add contact",
  role: "Add role",
};

const createRecord = (type: CrmKind, v: Record<string, string>) => {
  switch (type) {
    case "candidate":
      return RecordService.CreateCandidate({
        fullName: v.fullName,
        preferredName: v.preferredName,
        emails: list(v.emails),
        phones: list(v.phones),
        location: v.location,
        workRights: v.workRights,
        availability: v.availability,
        desiredEmploymentType: v.desiredEmploymentType,
        desiredWorkArrangement: v.desiredWorkArrangement,
        compensation: compensation(v),
        sourceNote: v.sourceNote,
        lastConfirmed: v.lastConfirmed,
      } as unknown as Candidate);
    case "company":
      return RecordService.CreateCompany({
        name: v.name,
        website: v.website,
        location: v.location,
        source: v.source,
      } as unknown as Company);
    case "contact":
      return RecordService.CreateContact({
        companyId: v.companyId ? Number(v.companyId) : 0,
        fullName: v.fullName,
        title: v.title,
        email: v.email,
        phone: v.phone,
        source: v.source,
      } as unknown as Contact);
    case "role":
      return RecordService.CreateRole({
        title: v.title,
        companyName: v.companyName,
        companyId: v.companyId ? Number(v.companyId) : null,
        location: v.location,
        workArrangement: v.workArrangement,
        employmentType: v.employmentType,
        compensation: compensation(v),
        publishedOn: v.publishedOn,
        closingOn: v.closingOn,
        retrievedOn: v.retrievedOn,
        sourceId: v.sourceId,
        canonicalUrl: v.canonicalUrl,
        source: v.source,
        origin: v.origin,
        lifecycleState: v.lifecycleState,
      } as unknown as Role);
  }
};

const getRecord = async (type: CrmKind, id: number) => {
  switch (type) {
    case "candidate":
      return RecordService.GetCandidate(id);
    case "company":
      return RecordService.GetCompany(id);
    case "contact":
      return RecordService.GetContact(id);
    case "role":
      return RecordService.GetRole(id);
  }
};

const initialFor = (type: CrmKind, record: Record<string, unknown>): Record<string, string> => {
  switch (type) {
    case "candidate": {
      const c = record as unknown as Candidate;
      return {
        fullName: c.fullName,
        preferredName: c.preferredName,
        emails: joinList(c.emails),
        phones: joinList(c.phones),
        location: c.location,
        workRights: c.workRights,
        availability: c.availability,
        desiredEmploymentType: c.desiredEmploymentType,
        desiredWorkArrangement: c.desiredWorkArrangement,
        compMin: c.compensation?.min != null ? String(c.compensation.min) : "",
        compMax: c.compensation?.max != null ? String(c.compensation.max) : "",
        compCurrency: c.compensation?.currency ?? "",
        compPeriod: c.compensation?.period ?? "",
        sourceNote: c.sourceNote,
        lastConfirmed: c.lastConfirmed,
      };
    }
    case "company": {
      const c = record as unknown as Company;
      return { name: c.name, website: c.website, location: c.location, source: c.source };
    }
    case "contact": {
      const c = record as unknown as Contact;
      return {
        companyId: c.companyId ? String(c.companyId) : "",
        fullName: c.fullName,
        title: c.title,
        email: c.email,
        phone: c.phone,
        source: c.source,
      };
    }
    case "role": {
      const r = record as unknown as Role;
      return {
        title: r.title,
        companyName: r.companyName,
        companyId: r.companyId ? String(r.companyId) : "",
        location: r.location,
        workArrangement: r.workArrangement,
        employmentType: r.employmentType,
        compMin: r.compensation?.min != null ? String(r.compensation.min) : "",
        compMax: r.compensation?.max != null ? String(r.compensation.max) : "",
        compCurrency: r.compensation?.currency ?? "",
        compPeriod: r.compensation?.period ?? "",
        publishedOn: r.publishedOn,
        closingOn: r.closingOn,
        retrievedOn: r.retrievedOn,
        sourceId: r.sourceId,
        canonicalUrl: r.canonicalUrl,
        source: r.source,
        origin: r.origin,
        lifecycleState: r.lifecycleState,
      };
    }
  }
};

const updateRecord = (type: CrmKind, id: number, v: Record<string, string>) => {
  switch (type) {
    case "candidate":
      return RecordService.UpdateCandidate({
        id,
        fullName: v.fullName,
        preferredName: v.preferredName,
        emails: list(v.emails),
        phones: list(v.phones),
        location: v.location,
        workRights: v.workRights,
        availability: v.availability,
        desiredEmploymentType: v.desiredEmploymentType,
        desiredWorkArrangement: v.desiredWorkArrangement,
        compensation: compensation(v),
        sourceNote: v.sourceNote,
        lastConfirmed: v.lastConfirmed,
      } as unknown as Candidate);
    case "company":
      return RecordService.UpdateCompany({
        id,
        name: v.name,
        website: v.website,
        location: v.location,
        source: v.source,
      } as unknown as Company);
    case "contact":
      return RecordService.UpdateContact({
        id,
        companyId: v.companyId ? Number(v.companyId) : 0,
        fullName: v.fullName,
        title: v.title,
        email: v.email,
        phone: v.phone,
        source: v.source,
      } as unknown as Contact);
    case "role":
      return RecordService.UpdateRole({
        id,
        title: v.title,
        companyName: v.companyName,
        companyId: v.companyId ? Number(v.companyId) : null,
        location: v.location,
        workArrangement: v.workArrangement,
        employmentType: v.employmentType,
        compensation: compensation(v),
        publishedOn: v.publishedOn,
        closingOn: v.closingOn,
        retrievedOn: v.retrievedOn,
        sourceId: v.sourceId,
        canonicalUrl: v.canonicalUrl,
        source: v.source,
        origin: v.origin,
        lifecycleState: v.lifecycleState,
      } as unknown as Role);
  }
};

const titleFor = (type: CrmKind, record: Record<string, unknown> | null): string => {
  if (!record) return "";
  switch (type) {
    case "candidate":
      return (record as unknown as Candidate).fullName;
    case "company":
      return (record as unknown as Company).name;
    case "contact":
      return (record as unknown as Contact).fullName;
    case "role":
      return (record as unknown as Role).title;
  }
};

// The right pane for whichever record is selected: its editable details, the
// artifacts attached to it, its interaction history, and — for a candidate —
// the profile the rest of the app acts on.
function Detail(props: { sel: () => { type: CrmKind; id: number } }) {
  const [record, setRecord] = createSignal<Record<string, unknown> | null>(null);
  const [companies, setCompanies] = createSignal<Company[]>([]);
  const [timeline, setTimeline] = createSignal<TimelineEntry[]>([]);
  const [roles, setRoles] = createSignal<Role[]>([]);
  const [initiatives, setInitiatives] = createSignal<Initiative[]>([]);
  const [profile, setProfile] = createSignal<Profile | null>(null);

  const [kind, setKind] = createSignal("call");
  const [note, setNote] = createSignal("");
  const [occurredAt, setOccurredAt] = createSignal("");
  const [roleId, setRoleId] = createSignal("");
  const [initiativeId, setInitiativeId] = createSignal("");
  const [editingId, setEditingId] = createSignal<number | null>(null);

  // The backend's own words, verbatim: it knows rules the UI does not.
  const { act, busy, error } = createAction();

  const resetLogForm = () => {
    setKind("call");
    setNote("");
    setOccurredAt("");
    setRoleId("");
    setInitiativeId("");
    setEditingId(null);
  };

  const loadTimeline = async () => {
    const s = props.sel();
    setTimeline(((await InteractionService.Timeline(s.type as LinkTarget, s.id)) ?? []) as TimelineEntry[]);
  };

  const load = () =>
    act(async () => {
      const s = props.sel();
      resetLogForm();
      const [rec, rolesList, initiativesList] = await Promise.all([
        getRecord(s.type, s.id),
        RecordService.ListRoles(),
        InitiativeService.List(false),
      ]);
      setRecord((rec as unknown as Record<string, unknown>) ?? null);
      setRoles((rolesList ?? []) as Role[]);
      setInitiatives((initiativesList ?? []) as Initiative[]);
      await loadTimeline();
      setProfile(s.type === "candidate" ? (((await CandidateProfileService.InUse(s.id)) ?? null) as Profile | null) : null);
    });

  createEffect(() => {
    props.sel();
    void load();
  });

  onMount(() => {
    void RecordService.ListCompanies().then((cs) => setCompanies((cs ?? []) as Company[]));
  });

  const submitDetails = async (v: Record<string, string>) => {
    const s = props.sel();
    await updateRecord(s.type, s.id, v);
    const rec = await getRecord(s.type, s.id);
    setRecord((rec as unknown as Record<string, unknown>) ?? null);
  };

  const submitInteraction = (e: Event) => {
    e.preventDefault();
    void act(async () => {
      const s = props.sel();
      const input = {
        id: editingId() ?? 0,
        targetType: s.type as LinkTarget,
        targetId: s.id,
        kind: kind(),
        note: note(),
        occurredAt: occurredAt(),
        // The role select is optional for every kind now, and required only
        // for outcome kinds — a role cleared by the recruiter sends 0, the
        // backend's "none" value.
        roleId: roleId() ? Number(roleId()) : 0,
        initiativeId: initiativeId() ? Number(initiativeId()) : 0,
      };
      if (editingId() !== null) await InteractionService.Update(input);
      else await InteractionService.Log(input);
      resetLogForm();
      await loadTimeline();
    });
  };

  const editEntry = (entry: TimelineEntry) => {
    setEditingId(entry.id);
    setKind(entry.kind);
    setNote(entry.note);
    setOccurredAt(entry.occurredAt);
    setRoleId(entry.roleId ? String(entry.roleId) : "");
    setInitiativeId(entry.initiativeId ? String(entry.initiativeId) : "");
  };

  const deleteEntry = (entry: TimelineEntry) =>
    act(async () => {
      await InteractionService.Delete(entry.id);
      await loadTimeline();
    });

  const companyOptions = () => [
    { value: "", label: "— none —" },
    ...companies().map((c) => ({ value: String(c.id), label: c.name })),
  ];

  const fields = () => {
    const specs = detailFields(props.sel().type);
    const t = props.sel().type;
    if (t === "contact" || t === "role") {
      return specs.map((f) => (f.key === "companyId" ? { ...f, options: companyOptions() } : f));
    }
    return specs;
  };

  return (
    <div class="crm-record">
      <h3>{titleFor(props.sel().type, record())}</h3>

      <Show when={error()}>
        <p class="modal-error" role="alert">{error()}</p>
      </Show>

      {/* keyed: a plain (non-keyed) Show only re-invokes its child on a
          falsy<->truthy transition of `record()`, so RecordForm would stay
          mounted — and frozen on its first initial values — across a change
          of selected record, or after a save refetches the same record.
          Keying on the record's own identity forces a fresh RecordForm
          instance, with fresh `initial` values, every time either happens. */}
      <Show when={record()} keyed>
        {(rec) => (
          <RecordForm
            legend="Details form"
            fields={fields()}
            submitLabel="Save"
            initial={initialFor(props.sel().type, rec)}
            onSubmit={submitDetails}
          />
        )}
      </Show>

      <ArtifactsPanel target={{ type: props.sel().type as LinkTarget, id: props.sel().id }} />

      <section class="record-section" aria-label="History">
        <h3>History</h3>
        <ul class="record-list" aria-label="Interaction history">
          <For each={timeline()} fallback={<li class="muted">No interactions logged yet.</li>}>
            {(entry) => (
              <li class="search-hit">
                <span class="artifact-name">
                  {entry.kind} — {entry.occurredAt}
                </span>
                <Show when={entry.roleTitle}>
                  <span class="muted"> {entry.roleTitle}</span>
                </Show>
                <Show when={entry.initiativeName}>
                  <span class="muted"> {entry.initiativeName}</span>
                </Show>
                {/* The recruiter's own words: displayed, never rendered. */}
                <p class="muted">{entry.note}</p>
                <button aria-label={`Edit interaction ${entry.id}`} onClick={() => editEntry(entry)}>
                  Edit
                </button>
                <button aria-label={`Delete interaction ${entry.id}`} onClick={() => deleteEntry(entry)}>
                  Delete
                </button>
              </li>
            )}
          </For>
        </ul>

        <form aria-label="Log interaction form" onSubmit={submitInteraction}>
          <label>
            <span>Kind</span>
            <select aria-label="Interaction kind" value={kind()} onChange={(e) => setKind(e.currentTarget.value)}>
              <For each={INTERACTION_KINDS}>{(k) => <option value={k.value}>{k.label}</option>}</For>
            </select>
          </label>
          <label>
            <span>Role</span>
            <select
              aria-label="Interaction role"
              value={roleId()}
              required={OUTCOME_KINDS.has(kind())}
              onChange={(e) => setRoleId(e.currentTarget.value)}
            >
              <option value="">No role</option>
              <For each={roles()}>{(r) => <option value={String(r.id)}>{r.title}</option>}</For>
            </select>
          </label>
          <label>
            <span>Initiative</span>
            <select aria-label="Interaction initiative" value={initiativeId()} onChange={(e) => setInitiativeId(e.currentTarget.value)}>
              <option value="">No initiative</option>
              <For each={initiatives()}>{(i) => <option value={String(i.id)}>{i.name}</option>}</For>
            </select>
          </label>
          <label>
            <span>Note</span>
            <textarea aria-label="Interaction note" value={note()} onInput={(e) => setNote(e.currentTarget.value)} />
          </label>
          <label>
            <span>Date</span>
            <input
              type="date"
              aria-label="Interaction date"
              value={occurredAt()}
              onInput={(e) => setOccurredAt(e.currentTarget.value)}
            />
          </label>
          <button class="primary" type="submit" disabled={busy()}>
            {editingId() !== null ? "Save interaction" : "Log interaction"}
          </button>
        </form>
      </section>

      <Show when={props.sel().type === "candidate"}>
        <section class="record-section" aria-label="Profile">
          <h3>Profile</h3>
          <Show when={profile()} fallback={<p class="muted">No approved profile yet.</p>}>
            {(p) => (
              <ul class="record-list" aria-label="Profile aspects">
                <For each={p().aspects ?? []} fallback={<li class="muted">Nothing here yet.</li>}>
                  {(aspect) => (
                    <li class="search-hit">
                      <span class="artifact-name">{aspect.type}</span>
                      {/* The candidate's own document, or an AI's reading of it: displayed, never rendered. */}
                      <pre>{aspect.wording}</pre>
                    </li>
                  )}
                </For>
              </ul>
            )}
          </Show>
        </section>
      </Show>
    </div>
  );
}

export default function CrmPanel() {
  const [kind, setKind] = createSignal<CrmKind>("candidate");
  const [filter, setFilter] = createSignal("");
  const [applied, setApplied] = createSignal("");
  const [selected, setSelected] = createSignal<{ type: CrmKind; id: number } | null>(null);
  const [people, setPeople] = createSignal<PersonHit[] | null>(null);
  const [talentQuery, setTalentQuery] = createSignal("");
  const [records, setRecords] = createSignal<Row[]>([]);
  const [creating, setCreating] = createSignal(false);
  const [companies, setCompanies] = createSignal<Company[]>([]);
  // The backend's own words, verbatim: it knows rules the UI does not — a
  // rejected search must land in error() and never throw through render.
  const { act, reloader, error } = createAction();

  const reload = reloader(async (isCurrent) => {
    const found = await rows(kind(), applied());
    if (!isCurrent()) return;
    setRecords(found);
  });

  createEffect(() => {
    kind();
    applied();
    void reload();
  });

  onMount(() => {
    void RecordService.ListCompanies().then((cs) => setCompanies((cs ?? []) as Company[]));
  });

  const runTalent = (e: Event) => {
    e.preventDefault();
    void act(async () => {
      setPeople(((await SearchService.People(talentQuery(), 20)) ?? []) as PersonHit[]);
    });
  };

  const companyOptions = () => [
    { value: "", label: "— none —" },
    ...companies().map((c) => ({ value: String(c.id), label: c.name })),
  ];

  const createFields = () => {
    const specs = detailFields(kind());
    if (kind() === "contact" || kind() === "role") {
      return specs.map((f) => (f.key === "companyId" ? { ...f, options: companyOptions() } : f));
    }
    return specs;
  };

  // Not wrapped in act: RecordForm already catches a rejected onSubmit and
  // shows the backend's message itself — inline under the field its `match`
  // names, or as its own role="alert" otherwise — the same way Detail's
  // submitDetails leaves record edits to RecordForm.
  const submitCreate = async (v: Record<string, string>) => {
    await createRecord(kind(), v);
    setCreating(false);
    await reload();
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
                  setCreating(false);
                }}
              >
                {k.label}
              </button>
            )}
          </For>
        </div>

        <button aria-label={`New ${kind()}`} onClick={() => setCreating(true)}>
          New {kind()}
        </button>

        <Show when={creating()}>
          <RecordForm
            legend={`New ${kind()}`}
            fields={createFields()}
            submitLabel={CREATE_LABELS[kind()]}
            onSubmit={submitCreate}
          />
          <button class="muted" onClick={() => setCreating(false)}>
            Cancel
          </button>
        </Show>

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
              <For each={records()}>
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
            <>
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
              </ul>
              <button class="muted" onClick={() => setPeople(null)}>
                Back to the list
              </button>
            </>
          )}
        </Show>
      </aside>

      <section class="crm-detail" aria-label="Record detail">
        <Show when={selected()} fallback={<p class="muted">Select a record to see its details and history.</p>}>
          {(sel) => <Detail sel={sel} />}
        </Show>
      </section>
    </div>
  );
}
