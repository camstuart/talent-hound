import { createEffect, createSignal, For, Show } from "solid-js";
import { bumpWorkspace, workspaceRevision } from "../workspaceRevision";
import { latestOnly } from "../latestOnly";
import { RecordService } from "../../bindings/camstuart/talent-hound";
import type { Candidate, Company, Contact, Role } from "../../bindings/camstuart/talent-hound/internal/models";
import RecordForm from "./RecordForm";
import { detailFields, payloadFor, withCompanies } from "./recordFields";

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
          fields={detailFields("candidate")}
          onSubmit={async (v) => {
            await RecordService.CreateCandidate(payloadFor("candidate", v) as unknown as Candidate);
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
          fields={detailFields("company")}
          onSubmit={async (v) => {
            await RecordService.CreateCompany(payloadFor("company", v) as unknown as Company);
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
          fields={withCompanies(detailFields("contact"), companyOptions())}
          onSubmit={async (v) => {
            await RecordService.CreateContact(payloadFor("contact", v) as unknown as Contact);
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
          fields={withCompanies(detailFields("role"), companyOptions())}
          onSubmit={async (v) => {
            await RecordService.CreateRole(payloadFor("role", v) as unknown as Role);
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
