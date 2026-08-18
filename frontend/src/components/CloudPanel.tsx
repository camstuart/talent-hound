import { createAction } from "../act";
import { createEffect, createSignal, For, Show } from "solid-js";
import { CloudService } from "../../bindings/camstuart/talent-hound";
import type { CloudEndpoint, Payload, TaskState } from "../../bindings/camstuart/talent-hound";
import { workspaceRevision } from "../workspaceRevision";

// The one deliberate exception to running locally, and the screen that makes it
// hard for the exception to become the rule.
//
// The permanently denied tasks are listed alongside the approvable ones on
// purpose: a screen that shows only what is off invites someone to look for the
// switch that turns on what is forbidden.
const TASK_LABELS: Record<string, string> = {
  role_extraction: "Reading public role listings",
  assessment: "Assessing matches",
  drafting: "Writing drafts",
  chat: "Answering questions",
  candidate_extraction: "Building Candidate Profiles",
  embedding: "Embedding evidence",
  raw_artifact: "Sending raw candidate documents",
};

export default function CloudPanel(props: { initiativeId: number }) {
  const [endpoint, setEndpoint] = createSignal<CloudEndpoint | null>(null);
  const [url, setUrl] = createSignal("");
  const [model, setModel] = createSignal("");
  // Set once the recruiter edits either field, so reloads stop seeding them.
  const [touched, setTouched] = createSignal(false);
  const [tasks, setTasks] = createSignal<TaskState[]>([]);
  const [preview, setPreview] = createSignal<Payload | null>(null);
  // The backend's own words, verbatim: it knows rules the UI does not.
  const { act, error, busy } = createAction();

  const reload = () =>
    act(async () => {
      const current = ((await CloudService.Endpoint()) ?? null) as CloudEndpoint | null;
      setEndpoint(current);
      // Seeded, not overwritten: a reload triggered by some other action must
      // not throw away what the recruiter is halfway through typing.
      if (current && !touched()) {
        setUrl(current.url);
        setModel(current.model);
      }
      setTasks(((await CloudService.Tasks(props.initiativeId)) ?? []) as TaskState[]);
    });

  createEffect(() => {
    workspaceRevision();
    void reload();
  });

  const configure = () =>
    act(async () => {
      await CloudService.Configure(url(), model());
      setPreview(null);
      // Saved: the fields may follow the stored endpoint again.
      setTouched(false);
      await reload();
    });

  const remove = () =>
    act(async () => {
      await CloudService.Remove();
      setPreview(null);
      await reload();
    });

  const approve = (task: string) =>
    act(async () => {
      await CloudService.Approve(props.initiativeId, task);
      await reload();
    });

  const revoke = (task: string) =>
    act(async () => {
      await CloudService.Revoke(props.initiativeId, task);
      await reload();
    });

  // The payload, not a description of it — the recruiter sees the bytes before
  // anything is sent.
  const showPayload = (task: string) =>
    act(async () => {
      setPreview(
        ((await CloudService.Preview({
          initiativeId: props.initiativeId,
          candidateId: 0,
          task,
          text: "This is the text that would be sent for this task.",
        } as never)) ?? null) as Payload | null,
      );
    });

  return (
    <section class="record-section" aria-label="Cloud">
      <h3>Cloud</h3>
      <p class="muted">
        Optional, per task, per initiative. Everything runs locally unless you approve a task here, and some things
        never can.
      </p>

      <div class="search-bar">
        <input
          aria-label="Cloud endpoint URL"
          placeholder="https://…"
          value={url()}
          onInput={(e) => {
            setTouched(true);
            setUrl(e.currentTarget.value);
          }}
        />
        <input
          aria-label="Cloud model"
          placeholder="model name"
          value={model()}
          onInput={(e) => {
            setTouched(true);
            setModel(e.currentTarget.value);
          }}
        />
        <button class="primary" aria-label="Save the cloud endpoint" disabled={busy()} onClick={configure}>
          Save
        </button>
        <Show when={endpoint()}>
          <button aria-label="Remove the cloud endpoint" onClick={remove}>
            Remove
          </button>
        </Show>
      </div>

      <Show when={endpoint()}>
        {(e) => (
          <p class="muted" aria-label="Cloud endpoint state">
            {e().url} — revision {e().revision}. Changing it clears every approval.
          </p>
        )}
      </Show>

      <Show when={error()}>
        <p class="modal-error">{error()}</p>
      </Show>

      <ul class="record-list" aria-label="Cloud tasks">
        <For each={tasks()} fallback={<li class="muted">No tasks.</li>}>
          {(task) => (
            <li class="search-hit">
              <span class="artifact-name">
                {TASK_LABELS[task.task] ?? task.task}
                <span class="muted">
                  {" "}
                  — {task.denied ? "never" : task.approved ? "approved" : "not approved"}
                </span>
              </span>
              <Show when={task.reason}>
                <p class={task.denied ? "modal-error" : "muted"} aria-label={`Why ${task.task} is not in use`}>
                  {task.reason}
                </p>
              </Show>
              <Show when={!task.denied}>
                <Show
                  when={task.approved}
                  fallback={
                    <button aria-label={`Approve ${task.task}`} onClick={() => approve(task.task)}>
                      Approve for this initiative
                    </button>
                  }
                >
                  <button aria-label={`Revoke ${task.task}`} onClick={() => revoke(task.task)}>
                    Revoke
                  </button>
                </Show>
                <button aria-label={`Preview the payload for ${task.task}`} onClick={() => showPayload(task.task)}>
                  Preview the payload
                </button>
              </Show>
            </li>
          )}
        </For>
      </ul>

      <Show when={preview()}>
        {(p) => (
          <div class="extraction-view" role="region" aria-label="Payload preview">
            <h4>
              This is exactly what would be sent
              <button aria-label="Close the payload preview" onClick={() => setPreview(null)}>
                Close
              </button>
            </h4>
            <p class="muted">
              {p().task} → {p().endpoint} ({p().model})
            </p>
            {/* Identifiers are already replaced: substitution happens before the
                preview, so this is what actually leaves. */}
            <pre aria-label="Payload text">{p().text}</pre>
          </div>
        )}
      </Show>
    </section>
  );
}
