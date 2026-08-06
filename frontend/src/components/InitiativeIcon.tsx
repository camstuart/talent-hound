import { InitiativeType } from "../../bindings/camstuart/talent-hound/internal/models";

export const INITIATIVE_TYPE_LABELS: Record<string, string> = {
  [InitiativeType.InitiativeTypeJobSearch]: "Job Search",
  [InitiativeType.InitiativeTypeTalentSearch]: "Talent Search",
  [InitiativeType.InitiativeTypeBusinessDevelopment]: "Business Development",
};

// One distinctive icon per initiative type, used in both the sidebar and tabs.
export function InitiativeIcon(props: { type: InitiativeType }) {
  const common = {
    class: "initiative-icon",
    width: 16,
    height: 16,
    viewBox: "0 0 24 24",
    fill: "none",
    stroke: "currentColor",
    "stroke-width": 2,
    "stroke-linecap": "round" as const,
    "stroke-linejoin": "round" as const,
    "aria-hidden": true,
  };

  switch (props.type) {
    case InitiativeType.InitiativeTypeJobSearch:
      // Briefcase
      return (
        <svg {...common} data-icon="job_search">
          <rect x="2" y="7" width="20" height="13" rx="2" />
          <path d="M16 7V5a2 2 0 0 0-2-2h-4a2 2 0 0 0-2 2v2" />
          <path d="M2 12h20" />
        </svg>
      );
    case InitiativeType.InitiativeTypeTalentSearch:
      // Person with magnifier
      return (
        <svg {...common} data-icon="talent_search">
          <circle cx="9" cy="7" r="3.5" />
          <path d="M3 20c0-3.3 2.7-6 6-6 1.2 0 2.3.35 3.2.95" />
          <circle cx="16.5" cy="16.5" r="3" />
          <path d="m21 21-2.4-2.4" />
        </svg>
      );
    default:
      // Business development: trending-up chart
      return (
        <svg {...common} data-icon="business_development">
          <path d="m3 17 6-6 4 4 7-7" />
          <path d="M14 8h6v6" />
        </svg>
      );
  }
}
