import { For, Show } from "solid-js";
import type { InitiativeType } from "../../bindings/camstuart/talent-hound/internal/models";
import { InitiativeIcon } from "./InitiativeIcon";

// One open tab: an initiative by id, or a utility screen by name.
export type TabId = number | "settings" | "help" | "crm";

export interface Tab {
  id: TabId;
  title: string;
  // Present only for initiative tabs; utility tabs have no type icon.
  type?: InitiativeType;
  archived?: boolean;
}

interface Props {
  tabs: Tab[];
  activeId: TabId | null;
  onActivate: (id: TabId) => void;
  onClose: (id: TabId) => void;
}

export default function TabBar(props: Props) {
  return (
    <div class="tabbar" role="tablist" aria-label="Open tabs">
      <For each={props.tabs}>
        {(tab) => (
          <div
            class="tab"
            classList={{ active: props.activeId === tab.id }}
            role="tab"
            aria-selected={props.activeId === tab.id}
            onClick={() => props.onActivate(tab.id)}
          >
            <Show when={tab.type}>{(type) => <InitiativeIcon type={type()} />}</Show>
            <span class="tab-title">{tab.title}</span>
            <Show when={tab.archived}>
              <span class="archived-badge">Archived</span>
            </Show>
            <button
              class="tab-close"
              aria-label={`Close ${tab.title}`}
              onClick={(e) => {
                e.stopPropagation();
                props.onClose(tab.id);
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
