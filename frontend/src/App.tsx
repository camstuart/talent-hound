import { createSignal, onMount, Show } from "solid-js";
import { InitiativeService } from "../bindings/camstuart/talent-hound";
import { InitiativeStatus } from "../bindings/camstuart/talent-hound/internal/models";
import type { Initiative, InitiativeType } from "../bindings/camstuart/talent-hound/internal/models";
import Sidebar from "./components/Sidebar";
import TabBar from "./components/TabBar";
import NewInitiativeModal from "./components/NewInitiativeModal";
import WorkspaceAreas from "./components/WorkspaceAreas";
import SettingsPanel from "./components/SettingsPanel";
import { InitiativeIcon, INITIATIVE_TYPE_LABELS } from "./components/InitiativeIcon";

export default function App() {
  const [initiatives, setInitiatives] = createSignal<Initiative[]>([]);
  const [openTabIds, setOpenTabIds] = createSignal<number[]>([]);
  const [activeId, setActiveId] = createSignal<number | null>(null);
  const [showModal, setShowModal] = createSignal(false);
  const [showArchived, setShowArchived] = createSignal(false);
  const [renaming, setRenaming] = createSignal(false);
  const [showSettings, setShowSettings] = createSignal(false);
  const [error, setError] = createSignal("");

  // The backend decides what "listed" means; the checkbox only asks a different
  // question. Archived initiatives are left out by default — except one that is
  // open, which stays on screen with its new label rather than vanishing.
  const reload = async (includeArchived = showArchived()) => {
    const listed = (await InitiativeService.List(includeArchived)) ?? [];
    const missing = openTabIds().filter((id) => !listed.some((i) => i.id === id));
    const stillOpen = await Promise.all(missing.map((id) => InitiativeService.Get(id).catch(() => null)));
    setInitiatives([...listed, ...stillOpen.filter((i): i is Initiative => i !== null)]);
  };
  onMount(() => reload());

  const byId = (id: number) => initiatives().find((i) => i.id === id);
  const openTabs = () => openTabIds().map(byId).filter((i): i is Initiative => i !== undefined);
  const activeInitiative = () => (activeId() !== null ? byId(activeId()!) : undefined);

  const openInitiative = (id: number) => {
    if (!openTabIds().includes(id)) setOpenTabIds([...openTabIds(), id]);
    setActiveId(id);
    setRenaming(false);
    setError("");
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

  const createInitiative = async (name: string, type: InitiativeType, candidateIDs: number[]) => {
    const created = await InitiativeService.Create(name, type, candidateIDs);
    if (created) {
      setInitiatives([...initiatives(), created]);
      openInitiative(created.id);
    }
    setShowModal(false);
  };

  // Lifecycle actions share one shape: run it, refresh the list, show whatever
  // the backend said if it refused.
  const act = async (run: () => Promise<unknown>) => {
    setError("");
    try {
      await run();
      await reload();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    }
  };

  const rename = async (id: number, name: string) => {
    setRenaming(false);
    if (!name.trim()) return;
    await act(() => InitiativeService.Rename(id, name.trim()));
  };

  const toggleArchived = async (show: boolean) => {
    setShowArchived(show);
    await reload(show);
  };

  return (
    <div class="app">
      <Sidebar
        initiatives={initiatives()}
        activeId={activeId()}
        showArchived={showArchived()}
        onSelect={openInitiative}
        onNew={() => setShowModal(true)}
        onToggleArchived={toggleArchived}
        onSettings={() => setShowSettings((on) => !on)}
      />
      <main class="main">
        <Show when={openTabs().length > 0}>
          <TabBar tabs={openTabs()} activeId={activeId()} onActivate={setActiveId} onClose={closeTab} />
        </Show>
        <div class="content">
          <Show when={showSettings()}>
            <SettingsPanel />
          </Show>
          <Show when={!showSettings()}>
          <Show when={activeInitiative()} fallback={<Welcome />}>
            {(initiative) => (
              <section class="initiative-panel">
                <header class="initiative-panel-header">
                  <InitiativeIcon type={initiative().type} />
                  <Show
                    when={renaming()}
                    fallback={
                      <>
                        <h1>{initiative().name}</h1>
                        <button onClick={() => setRenaming(true)}>Rename</button>
                      </>
                    }
                  >
                    <input
                      class="rename-input"
                      aria-label="New name"
                      value={initiative().name}
                      ref={(el) => setTimeout(() => el.select())}
                      onKeyDown={(e) => {
                        if (e.key === "Enter") rename(initiative().id, e.currentTarget.value);
                        if (e.key === "Escape") setRenaming(false);
                      }}
                      onBlur={(e) => rename(initiative().id, e.currentTarget.value)}
                    />
                  </Show>
                  <span class="initiative-type-badge">{INITIATIVE_TYPE_LABELS[initiative().type]}</span>
                  <span class="initiative-status-badge" data-status={initiative().status}>
                    {initiative().status === InitiativeStatus.InitiativeArchived ? "Archived" : "Active"}
                  </span>
                  {/* Exactly one of archive or reopen, per the current state. */}
                  <Show
                    when={initiative().status === InitiativeStatus.InitiativeArchived}
                    fallback={
                      <button onClick={() => act(() => InitiativeService.Archive(initiative().id))}>Archive</button>
                    }
                  >
                    <button onClick={() => act(() => InitiativeService.Reopen(initiative().id))}>Reopen</button>
                  </Show>
                </header>
                <Show when={error()}>
                  <p class="modal-error">{error()}</p>
                </Show>
                <WorkspaceAreas initiativeId={initiative().id} type={initiative().type} />
              </section>
            )}
          </Show>
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
