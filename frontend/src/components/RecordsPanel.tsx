import { createEffect, createSignal, For, Show } from "solid-js";
import { bumpWorkspace, workspaceRevision } from "../workspaceRevision";
import { latestOnly } from "../latestOnly";
import { RecordService } from "../../bindings/camstuart/talent-hound";
import type { Candidate, Company, Contact, Role } from "../../bindings/camstuart/talent-hound/internal/models";
import RecordForm, { list, num, type FieldSpec } from "./RecordForm";

const PERIODS = [
  { value: "", label: "—" },
  { value: "hour", label: "per hour" },
  { value: "day", label: "per day" },
  { value: "week", label: "per week" },
  { value: "month", label: "per month" },
  { value: "year", label: "per year" },
];

// Compensation is four fields wherever it appears.
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

// Shared records: candidates, companies, contacts, and roles, plus the warm-path
// contacts-at-company lookup. They belong to the talent pool, not to any one
// initiative, so this panel shows the same data in every workspace.
export default function RecordsPanel() {
  const [candidates, setCandidates] = createSignal<Candidate[]>([]);
  const [companies, setCompanies] = createSignal<Company[]>([]);
  const [roles, setRoles] = createSignal<Role[]>([]);
  const [selectedCompany, setSelectedCompany] = createSignal("");
  const [contactsAt, setContactsAt] = createSignal<{ count: number; contacts: Contact[] } | null>(null);
  const [lookupError, setLookupError] = createSignal("");

  // Only the newest reload may write, and a failed one retries once. Both
  // rules live in latestOnly, because every panel with more than one reason to
  // reload needs them and a panel that has only the first still shows a record
  // the database holds and the screen does not.
  const reload = latestOnly(async (isCurrent) => {
    const [candidates, companies, roles] = await Promise.all([
      RecordService.ListCandidates(),
      RecordService.ListCompanies(),
      RecordService.ListRoles(),
    ]);
    if (!isCurrent()) return;
    setCandidates(candidates ?? []);
    setCompanies(companies ?? []);
    setRoles(roles ?? []);
  });
  // Records are created elsewhere too — a new initiative can create its
  // candidate, and a dropped resume creates one — so this list follows the
  // workspace revision rather than only its own actions.
  createEffect(() => {
    workspaceRevision();
    void reload();
  });

  const companyOptions = () => [
    { value: "", label: "— none —" },
    ...companies().map((c) => ({ value: String(c.id), label: c.name })),
  ];

  const lookup = async (companyId: string) => {
    setSelectedCompany(companyId);
    setContactsAt(null);
    setLookupError("");
    if (!companyId) return;
    try {
      const at = await RecordService.ContactsAtCompany(Number(companyId));
      if (at) setContactsAt({ count: at.count, contacts: at.contacts ?? [] });
    } catch (err) {
      setLookupError(err instanceof Error ? err.message : String(err));
    }
  };

  return (
    <div class="records">
      <section class="record-section" aria-label="Candidates">
        <h3>Candidates</h3>
        <ul class="record-list">
          <For each={candidates()} fallback={<li class="muted">No candidates yet.</li>}>
            {(c) => (
              <li>
                {c.fullName}
                <Show when={c.location}>
                  <span class="muted"> — {c.location}</span>
                </Show>
              </li>
            )}
          </For>
        </ul>
        <RecordForm
          legend="New candidate"
          submitLabel="Add candidate"
          fields={[
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
          ]}
          onSubmit={async (v) => {
            await RecordService.CreateCandidate({
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
            // Other panels list these records too.
            // The bump reloads this panel too, and awaiting a second one here
            // would report a failed refresh as a failed create.
            bumpWorkspace();
          }}
        />
      </section>

      <section class="record-section" aria-label="Companies">
        <h3>Companies</h3>
        <ul class="record-list">
          <For each={companies()} fallback={<li class="muted">No companies yet.</li>}>
            {(c) => <li>{c.name}</li>}
          </For>
        </ul>
        <RecordForm
          legend="New company"
          submitLabel="Add company"
          fields={[
            { key: "name", label: "Name", required: true, match: "company name" },
            { key: "website", label: "Website", placeholder: "https://example.test", match: "website" },
            { key: "location", label: "Location" },
            { key: "source", label: "Source" },
          ]}
          onSubmit={async (v) => {
            await RecordService.CreateCompany({
              name: v.name,
              website: v.website,
              location: v.location,
              source: v.source,
            } as unknown as Company);
            // Other panels list these records too.
            // The bump reloads this panel too, and awaiting a second one here
            // would report a failed refresh as a failed create.
            bumpWorkspace();
          }}
        />
      </section>

      <section class="record-section" aria-label="Contacts">
        <h3>Contacts</h3>
        <RecordForm
          legend="New contact"
          submitLabel="Add contact"
          fields={[
            { key: "companyId", label: "Company", type: "select", options: companyOptions(), match: "company" },
            { key: "fullName", label: "Full name", required: true, match: "contact full name" },
            { key: "title", label: "Role or title" },
            { key: "email", label: "Email" },
            { key: "phone", label: "Phone" },
            { key: "source", label: "Source" },
          ]}
          onSubmit={async (v) => {
            await RecordService.CreateContact({
              companyId: Number(v.companyId),
              fullName: v.fullName,
              title: v.title,
              email: v.email,
              phone: v.phone,
              source: v.source,
            } as unknown as Contact);
            await lookup(selectedCompany());
          }}
        />

        <div class="contacts-at-company">
          <label class="record-field">
            <span>Contacts at company</span>
            <select
              aria-label="Contacts at company"
              value={selectedCompany()}
              onChange={(e) => lookup(e.currentTarget.value)}
            >
              <For each={companyOptions()}>{(o) => <option value={o.value}>{o.label}</option>}</For>
            </select>
          </label>
          <Show when={lookupError()}>
            <p class="modal-error" role="alert">{lookupError()}</p>
          </Show>
          <Show when={contactsAt()}>
            {(at) => (
              <div>
                <p data-testid="contacts-count">
                  {at().count} known {at().count === 1 ? "contact" : "contacts"}
                </p>
                <ul class="record-list">
                  <For each={at().contacts} fallback={<li class="muted">Nobody we know there yet.</li>}>
                    {(c) => (
                      <li>
                        {c.fullName}
                        <Show when={c.title}>
                          <span class="muted"> — {c.title}</span>
                        </Show>
                      </li>
                    )}
                  </For>
                </ul>
              </div>
            )}
          </Show>
        </div>
      </section>

      <section class="record-section" aria-label="Roles">
        <h3>Roles</h3>
        <ul class="record-list">
          <For each={roles()} fallback={<li class="muted">No roles yet.</li>}>
            {(r) => (
              <li>
                {r.title}
                <Show when={r.companyName}>
                  <span class="muted"> — {r.companyName}</span>
                </Show>
              </li>
            )}
          </For>
        </ul>
        <RecordForm
          legend="New role"
          submitLabel="Add role"
          fields={[
            { key: "title", label: "Title", required: true, match: "role title" },
            { key: "companyName", label: "Company name" },
            { key: "companyId", label: "Company record", type: "select", options: companyOptions() },
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
          ]}
          onSubmit={async (v) => {
            await RecordService.CreateRole({
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
            // Other panels list these records too.
            // The bump reloads this panel too, and awaiting a second one here
            // would report a failed refresh as a failed create.
            bumpWorkspace();
          }}
        />
      </section>
    </div>
  );
}
