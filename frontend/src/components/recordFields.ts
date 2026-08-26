import { list, num, type FieldSpec } from "./RecordForm";

// The four record types and their forms, in one place: a record created in
// the workspace and edited in the CRM must show exactly the same fields, and
// two copies of a field list are two lists that drift.
export type CrmKind = "candidate" | "company" | "contact" | "role";

export const PERIODS = [
  { value: "", label: "—" },
  { value: "hour", label: "per hour" },
  { value: "day", label: "per day" },
  { value: "week", label: "per week" },
  { value: "month", label: "per month" },
  { value: "year", label: "per year" },
];

// Compensation is four fields wherever it appears.
export const compensationFields = (): FieldSpec[] => [
  { key: "compMin", label: "Compensation minimum", type: "number", match: "minimum" },
  { key: "compMax", label: "Compensation maximum", type: "number", match: "maximum" },
  { key: "compCurrency", label: "Currency", placeholder: "NZD", match: "currency" },
  { key: "compPeriod", label: "Period", type: "select", options: PERIODS, match: "period" },
];

export const compensation = (v: Record<string, string>) => ({
  min: num(v.compMin),
  max: num(v.compMax),
  currency: v.compCurrency,
  period: v.compPeriod,
});

export type Option = { value: string; label: string };

// detailFields is the form for one record type. Company pickers start with no
// options; the caller supplies the companies it knows with withCompanies.
export const detailFields = (kind: CrmKind): FieldSpec[] => {
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

// withCompanies fills the company picker of a contact or role form.
export const withCompanies = (specs: FieldSpec[], options: Option[]): FieldSpec[] =>
  specs.map((f) => (f.key === "companyId" ? { ...f, options } : f));

// payloadFor turns form values into the record the backend expects. The id is
// present for an update and absent for a create; nothing else differs.
export const payloadFor = (kind: CrmKind, v: Record<string, string>, id?: number): Record<string, unknown> => {
  const base = id === undefined ? {} : { id };
  switch (kind) {
    case "candidate":
      return {
        ...base,
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
      };
    case "company":
      return { ...base, name: v.name, website: v.website, location: v.location, source: v.source };
    case "contact":
      return {
        ...base,
        companyId: v.companyId ? Number(v.companyId) : 0,
        fullName: v.fullName,
        title: v.title,
        email: v.email,
        phone: v.phone,
        source: v.source,
      };
    case "role":
      return {
        ...base,
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
      };
  }
};
