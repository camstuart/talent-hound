import { For, Show } from "solid-js";
import { InitiativeStatus } from "../../bindings/camstuart/talent-hound/internal/models";
import type { Initiative } from "../../bindings/camstuart/talent-hound/internal/models";
import { InitiativeIcon } from "./InitiativeIcon";

interface Props {
  initiatives: Initiative[];
  activeId: number | null;
  showArchived: boolean;
  onSelect: (id: number) => void;
  onNew: () => void;
  onToggleArchived: (show: boolean) => void;
  onSettings: () => void;
}

export default function Sidebar(props: Props) {
  return (
    <nav class="sidebar" aria-label="Initiatives">
      <div class="sidebar-header">
        <span class="sidebar-title">Initiatives</span>
        <button class="icon-button" aria-label="New initiative" title="New initiative" onClick={() => props.onNew()}>
          +
        </button>
      </div>
      <label class="sidebar-filter">
        <input
          type="checkbox"
          checked={props.showArchived}
          onChange={(e) => props.onToggleArchived(e.currentTarget.checked)}
        />
        Show archived
      </label>
      <ul class="initiative-list">
        <For each={props.initiatives}>
          {(initiative) => (
            <li>
              <button
                class="initiative-item"
                classList={{ active: props.activeId === initiative.id }}
                onClick={() => props.onSelect(initiative.id)}
              >
                <InitiativeIcon type={initiative.type} />
                <span class="initiative-name">{initiative.name}</span>
                {/* Archived initiatives are labelled without being opened. */}
                <Show when={initiative.status === InitiativeStatus.InitiativeArchived}>
                  <span class="archived-badge">Archived</span>
                </Show>
              </button>
            </li>
          )}
        </For>
      </ul>
      {/* Model roles and provider keys are application-wide, not per workspace. */}
      <button class="sidebar-settings" aria-label="Settings" onClick={() => props.onSettings()}>
        Settings
      </button>
    </nav>
  );
}
