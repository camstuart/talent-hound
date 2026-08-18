import { createSignal, For, Show } from "solid-js";
import { InitiativeType } from "../../bindings/camstuart/talent-hound/internal/models";
import RecordsPanel from "./RecordsPanel";
import ArtifactsPanel from "./ArtifactsPanel";
import JobsPanel from "./JobsPanel";
import SearchPanel from "./SearchPanel";
import CandidateProfilePanel from "./CandidateProfilePanel";
import RoleProfilePanel from "./RoleProfilePanel";
import CriteriaPanel from "./CriteriaPanel";
import DiscoveryPanel from "./DiscoveryPanel";
import ShortlistPanel from "./ShortlistPanel";
import MatchesPanel from "./MatchesPanel";
import DraftsPanel from "./DraftsPanel";
import CloudPanel from "./CloudPanel";
import DeletePanel from "./DeletePanel";

// The four areas every initiative workspace has. Each one says what will live
// there; none of them pretends to a pipeline that does not exist yet.
const AREAS = [
  {
    key: "context",
    label: "Context",
    blurb: "The candidate profile, role profiles, search criteria, records, and artifacts for this initiative.",
  },
  { key: "research", label: "Research", blurb: "Discovered roles, their profiles, and the searches that found them." },
  { key: "matches", label: "Matches", blurb: "Two-directional assessments between candidates and roles, with their evidence." },
  { key: "drafts", label: "Drafts", blurb: "Outreach drafts, kept until they are copied out or discarded." },
] as const;

const SHELL_NOTE: Partial<Record<InitiativeType, string>> = {
  [InitiativeType.InitiativeTypeTalentSearch]:
    "Talent Search is a workspace shell in this proof of concept: its pipeline is outside PoC scope.",
  [InitiativeType.InitiativeTypeBusinessDevelopment]:
    "Business Development is a workspace shell in this proof of concept: its pipeline is outside PoC scope.",
};

export default function WorkspaceAreas(props: { initiativeId: number; type: InitiativeType }) {
  const [area, setArea] = createSignal<string>(AREAS[0].key);
  const current = () => AREAS.find((a) => a.key === area()) ?? AREAS[0];

  return (
    <div class="areas">
      <div class="area-tabs" role="tablist" aria-label="Initiative areas">
        <For each={AREAS}>
          {(a) => (
            <button
              class="area-tab"
              classList={{ active: area() === a.key }}
              role="tab"
              aria-selected={area() === a.key}
              onClick={() => setArea(a.key)}
            >
              {a.label}
            </button>
          )}
        </For>
      </div>
      <section class="area-panel" role="tabpanel" aria-label={current().label}>
        <Show when={SHELL_NOTE[props.type]}>
          {(note) => <p class="shell-note">{note()}</p>}
        </Show>
        <p class="muted">{current().blurb}</p>
        <Show when={area() === "context"}>
          <ArtifactsPanel initiativeId={props.initiativeId} />
          <CandidateProfilePanel initiativeId={props.initiativeId} />
          <CriteriaPanel initiativeId={props.initiativeId} />
          <SearchPanel initiativeId={props.initiativeId} />
          <JobsPanel initiativeId={props.initiativeId} />
          <RecordsPanel />
          <CloudPanel initiativeId={props.initiativeId} />
          <DeletePanel />
        </Show>
        <Show when={area() === "research"}>
          <DiscoveryPanel initiativeId={props.initiativeId} />
          <RoleProfilePanel initiativeId={props.initiativeId} />
        </Show>
        <Show when={area() === "drafts"}>
          <DraftsPanel initiativeId={props.initiativeId} />
        </Show>
        <Show when={area() === "matches"}>
          <ShortlistPanel initiativeId={props.initiativeId} />
          <MatchesPanel initiativeId={props.initiativeId} />
        </Show>
      </section>
    </div>
  );
}
