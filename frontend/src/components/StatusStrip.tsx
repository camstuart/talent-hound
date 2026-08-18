import { createSignal, createEffect, For, Show } from "solid-js";
import { CloudService, ModelService, SetupService } from "../../bindings/camstuart/talent-hound";
import type { ScopeState } from "../../bindings/camstuart/talent-hound";
import type { Status, TaskState } from "../../bindings/camstuart/talent-hound";
import { Scope } from "../../bindings/camstuart/talent-hound/internal/setup";

// What is true right now, kept on screen: which initiative, what scope, which
// models, whether a cloud override is in force, and whether anything local is
// actually reachable.
export default function StatusStrip(props: { initiativeId?: number; initiativeName?: string }) {
  const [setup, setSetup] = createSignal<ScopeState | null>(null);
  const [models, setModels] = createSignal<Status[]>([]);
  const [cloud, setCloud] = createSignal<TaskState[]>([]);

  const reload = async () => {
    // Each is independent: one unreachable dependency must not blank the rest
    // of the strip, which is exactly when the recruiter needs to read it.
    // Scope, not State: the strip redraws on every change, and State runs the
    // sidecar and Ollama checks.
    setSetup(((await SetupService.Scope().catch(() => null)) ?? null) as ScopeState | null);
    setModels(((await ModelService.Check().catch(() => [])) ?? []) as Status[]);
    if (props.initiativeId !== undefined) {
      setCloud(((await CloudService.Tasks(props.initiativeId).catch(() => [])) ?? []) as TaskState[]);
    } else {
      setCloud([]);
    }
  };

  // Deliberately not tied to the workspace revision: none of what the strip
  // shows changes when a candidate is added, and re-reading model availability
  // on every edit is a dependency check per keystroke.
  createEffect(() => {
    void props.initiativeId;
    void reload();
  });

  // Offline is a local fact: the models all live on this machine, so an
  // endpoint that cannot be reached is what "offline" means here.
  const online = () => models().some((m) => m.state === "ready");
  const overrides = () => cloud().filter((t) => t.approved);

  return (
    <footer class="status-strip" aria-label="Operating state">
      <span aria-label="Active initiative">
        {props.initiativeName ? props.initiativeName : "No initiative open"}
      </span>
      <span aria-label="Data scope">
        {setup()?.scope === Scope.ScopeDemo ? "Demo scope" : "Real scope"}
        {setup() && !setup()!.realData ? " (candidate data blocked)" : ""}
      </span>
      <span aria-label="Selected models">
        <For each={models()} fallback={"No models assigned"}>
          {(m, i) => (
            <>
              {i() > 0 ? " · " : ""}
              {m.role}: {m.model || "unassigned"} ({m.state})
            </>
          )}
        </For>
      </span>
      <span aria-label="Cloud override">
        <Show when={overrides().length > 0} fallback={"Local only"}>
          Cloud override in force: {overrides().map((t) => t.task).join(", ")}
        </Show>
      </span>
      <span aria-label="Connectivity">{online() ? "Local models reachable" : "Offline"}</span>
    </footer>
  );
}
