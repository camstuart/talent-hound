import { bumpWorkspace } from "./workspaceRevision";
import { createSignal, onMount, Show } from "solid-js";
import { InitiativeService } from "../bindings/camstuart/talent-hound";
import { InitiativeStatus } from "../bindings/camstuart/talent-hound/internal/models";
import type { Initiative, InitiativeType } from "../bindings/camstuart/talent-hound/internal/models";
import Sidebar from "./components/Sidebar";
import TabBar from "./components/TabBar";
import type { Tab, TabId } from "./components/TabBar";
import NewInitiativeModal from "./components/NewInitiativeModal";
import WorkspaceAreas from "./components/WorkspaceAreas";
import SettingsPanel from "./components/SettingsPanel";
import HelpPanel from "./components/HelpPanel";
import CrmPanel from "./components/CrmPanel";
import StatusStrip from "./components/StatusStrip";
import FirstRunWizard from "./components/FirstRunWizard";
import { SetupService } from "../bindings/camstuart/talent-hound";
import { InitiativeIcon, INITIATIVE_TYPE_LABELS } from "./components/InitiativeIcon";

export default function App() {
  const [initiatives, setInitiatives] = createSignal<Initiative[]>([]);
  const [openTabIds, setOpenTabIds] = createSignal<TabId[]>([]);
  const [activeId, setActiveId] = createSignal<TabId | null>(null);
  const [showModal, setShowModal] = createSignal(false);
  const [showArchived, setShowArchived] = createSignal(false);
  const [renaming, setRenaming] = createSignal(false);
  const [error, setError] = createSignal("");
  // Setup is only in the way while there is no data folder: with nowhere to put
  // anything, every other screen is a screen that cannot save what it collects.
  const [needsSetup, setNeedsSetup] = createSignal(false);

  // The backend decides what "listed" means; the checkbox only asks a different
  // question. Archived initiatives are left out by default — except one that is
  // open, which stays on screen with its new label rather than vanishing.
  const reload = async (includeArchived = showArchived()) => {
    const listed = (await InitiativeService.List(includeArchived)) ?? [];
    const missing = openTabIds().filter(
      (id): id is number => typeof id === "number" && !listed.some((i) => i.id === id),
    );
    const stillOpen = await Promise.all(missing.map((id) => InitiativeService.Get(id).catch(() => null)));
    setInitiatives([...listed, ...stillOpen.filter((i): i is Initiative => i !== null)]);
  };
  onMount(() => reload());
  onMount(async () => {
    const state = await SetupService.State().catch(() => null);
    setNeedsSetup(state?.next === "data_folder");
  });

  const byId = (id: number) => initiatives().find((i) => i.id === id);
  const UTILITY_TITLES = { settings: "Settings", help: "Help", crm: "CRM" } as const;
  const openTabs = (): Tab[] =>
    openTabIds()
      .map((id): Tab | undefined => {
        if (typeof id !== "number") return { id, title: UTILITY_TITLES[id] };
        const initiative = byId(id);
        if (!initiative) return undefined;
        return {
          id,
          title: initiative.name,
          type: initiative.type,
          archived: initiative.status === InitiativeStatus.InitiativeArchived,
        };
      })
      .filter((t): t is Tab => t !== undefined);
  const activeInitiative = () => (typeof activeId() === "number" ? byId(activeId() as number) : undefined);

  const openInitiative = (id: TabId) => {
    if (!openTabIds().includes(id)) setOpenTabIds([...openTabIds(), id]);
    setActiveId(id);
    setRenaming(false);
    setError("");
  };

  const closeTab = (id: TabId) => {
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
    // Creating a job search can create its candidate, which the records and
    // profile panels are showing.
    bumpWorkspace();
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
      {/* The window has no native title bar, so this strip is both the drag
          handle and the home of the application-wide menu items. Its left
          padding is where the macOS traffic lights sit. */}
      <header class="titlebar">
        <span class="titlebar-title">Talent Hound</span>
        <div class="titlebar-actions">
          <button aria-label="CRM" aria-pressed={activeId() === "crm"} onClick={() => openInitiative("crm")}>
            CRM
          </button>
          <button aria-label="Help" aria-pressed={activeId() === "help"} onClick={() => openInitiative("help")}>
            Help
          </button>
          <button
            aria-label="Settings"
            aria-pressed={activeId() === "settings"}
            onClick={() => openInitiative("settings")}
          >
            Settings
          </button>
        </div>
      </header>
      <div class="app-body">
        <Sidebar
          initiatives={initiatives()}
          activeId={activeInitiative()?.id ?? null}
          showArchived={showArchived()}
          onSelect={openInitiative}
          onNew={() => setShowModal(true)}
          onToggleArchived={toggleArchived}
        />
        <main class="main">
          <Show when={openTabs().length > 0}>
            <TabBar tabs={openTabs()} activeId={activeId()} onActivate={setActiveId} onClose={closeTab} />
          </Show>
          <div class="content">
            <Show when={needsSetup()}>
              <div class="container">
                <h1>Welcome to Talent Hound</h1>
                <p class="muted">Choose where this installation keeps its data before anything else.</p>
                <FirstRunWizard />
              </div>
            </Show>
            <Show when={!needsSetup() && activeId() === "help"}>
              <HelpPanel />
            </Show>
            <Show when={!needsSetup() && activeId() === "settings"}>
              <SettingsPanel />
            </Show>
            <Show when={!needsSetup() && activeId() === "crm"}>
              <CrmPanel />
            </Show>
            <Show when={!needsSetup() && typeof activeId() !== "string"}>
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
          <StatusStrip initiativeId={activeInitiative()?.id} initiativeName={activeInitiative()?.name} />
        </main>
      </div>
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
