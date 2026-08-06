import { For } from "solid-js";
import type { Initiative } from "../../bindings/camstuart/talent-hound/internal/models";
import { InitiativeIcon } from "./InitiativeIcon";

interface Props {
  initiatives: Initiative[];
  activeId: number | null;
  onSelect: (id: number) => void;
  onNew: () => void;
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
              </button>
            </li>
          )}
        </For>
      </ul>
    </nav>
  );
}
