import { createSignal, onMount, Show } from "solid-js";
import { InitiativeService } from "../bindings/camstuart/talent-hound";
import type { Initiative, InitiativeType } from "../bindings/camstuart/talent-hound/internal/models";
import Sidebar from "./components/Sidebar";
import TabBar from "./components/TabBar";
import NewInitiativeModal from "./components/NewInitiativeModal";
import { InitiativeIcon, INITIATIVE_TYPE_LABELS } from "./components/InitiativeIcon";

export default function App() {
  const [initiatives, setInitiatives] = createSignal<Initiative[]>([]);
  const [openTabIds, setOpenTabIds] = createSignal<number[]>([]);
  const [activeId, setActiveId] = createSignal<number | null>(null);
  const [showModal, setShowModal] = createSignal(false);

  onMount(async () => {
    setInitiatives((await InitiativeService.List()) ?? []);
  });

  const byId = (id: number) => initiatives().find((i) => i.id === id);
  const openTabs = () => openTabIds().map(byId).filter((i): i is Initiative => i !== undefined);
  const activeInitiative = () => (activeId() !== null ? byId(activeId()!) : undefined);

  const openInitiative = (id: number) => {
    if (!openTabIds().includes(id)) setOpenTabIds([...openTabIds(), id]);
    setActiveId(id);
  };

  const closeTab = (id: number) => {
    const ids = openTabIds();
    const idx = ids.indexOf(id);
    const remaining = ids.filter((tabId) => tabId !== id);
    setOpenTabIds(remaining);
    if (activeId() === id) {
      setActiveId(remaining.length ? remaining[Math.min(idx, remaining.length - 1)] : null);
    }
  };

  const createInitiative = async (name: string, type: InitiativeType) => {
    const created = await InitiativeService.Create(name, type);
    if (created) {
      setInitiatives([...initiatives(), created]);
      openInitiative(created.id);
    }
    setShowModal(false);
  };

  return (
    <div class="app">
      <Sidebar
        initiatives={initiatives()}
        activeId={activeId()}
        onSelect={openInitiative}
        onNew={() => setShowModal(true)}
      />
      <main class="main">
        <Show when={openTabs().length > 0}>
          <TabBar tabs={openTabs()} activeId={activeId()} onActivate={setActiveId} onClose={closeTab} />
        </Show>
        <div class="content">
          <Show when={activeInitiative()} fallback={<Welcome />}>
            {(initiative) => (
              <section class="initiative-panel">
                <header class="initiative-panel-header">
                  <InitiativeIcon type={initiative().type} />
                  <h1>{initiative().name}</h1>
                  <span class="initiative-type-badge">{INITIATIVE_TYPE_LABELS[initiative().type]}</span>
                </header>
                <p class="muted">
                  Created {new Date(initiative().createdAt).toLocaleString()}. This initiative doesn't do anything
                  yet — its workspace will live here.
                </p>
              </section>
            )}
          </Show>
        </div>
      </main>
      <Show when={showModal()}>
        <NewInitiativeModal onCreate={createInitiative} onCancel={() => setShowModal(false)} />
      </Show>
    </div>
  );
}

// Placeholder home screen shown when no initiative tab is open.
function Welcome() {
  return (
    <div class="container">
      <h1>Talent Hound</h1>
      <p class="muted">Create an initiative from the panel on the left to get started.</p>
    </div>
  );
}
