import { For, Show } from "solid-js";
import { InitiativeStatus } from "../../bindings/camstuart/talent-hound/internal/models";
import type { Initiative } from "../../bindings/camstuart/talent-hound/internal/models";
import { InitiativeIcon } from "./InitiativeIcon";

interface Props {
  tabs: Initiative[];
  activeId: number | null;
  onActivate: (id: number) => void;
  onClose: (id: number) => void;
}

export default function TabBar(props: Props) {
  return (
    <div class="tabbar" role="tablist" aria-label="Open initiatives">
      <For each={props.tabs}>
        {(initiative) => (
          <div
            class="tab"
            classList={{ active: props.activeId === initiative.id }}
            role="tab"
            aria-selected={props.activeId === initiative.id}
            onClick={() => props.onActivate(initiative.id)}
          >
            <InitiativeIcon type={initiative.type} />
            <span class="tab-title">{initiative.name}</span>
            <Show when={initiative.status === InitiativeStatus.InitiativeArchived}>
              <span class="archived-badge">Archived</span>
            </Show>
            <button
              class="tab-close"
              aria-label={`Close ${initiative.name}`}
              onClick={(e) => {
                e.stopPropagation();
                props.onClose(initiative.id);
              }}
            >
              ×
            </button>
          </div>
        )}
      </For>
    </div>
  );
}
