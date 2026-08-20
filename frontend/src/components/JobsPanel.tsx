import { createEffect, createSignal, For, onCleanup, Show } from "solid-js";
import { JobService } from "../../bindings/camstuart/talent-hound";
import { JobState } from "../../bindings/camstuart/talent-hound/internal/models";
import type { Job } from "../../bindings/camstuart/talent-hound/internal/models";

// How often the panel asks the backend where a running job has got to. Jobs
// live in the database, not in a channel, so polling is the honest reading —
// and it stops entirely once nothing is unfinished.
const POLL_MS = 400;

const UNFINISHED: JobState[] = [JobState.JobQueued, JobState.JobRunning];

const isUnfinished = (j: Job) => UNFINISHED.includes(j.state);

// Cancelled jobs are the one outcome the recruiter already knows about — they
// asked for it — so they get their own tab instead of pushing live work down
// the screen. A failure stays in the main list, because a failure is news.
export default function JobsPanel(props: { initiativeId: number }) {
  const [jobs, setJobs] = createSignal<Job[]>([]);
  const [tab, setTab] = createSignal<"active" | "cancelled">("active");
  const [error, setError] = createSignal("");

  const reload = async () => {
    setJobs((await JobService.ListForInitiative(props.initiativeId)) ?? []);
  };

  // Every action reports the backend's own words: it knows rules the UI does not.
  const act = async (run: () => Promise<unknown>) => {
    setError("");
    try {
      await run();
      await reload();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    }
  };

  createEffect(() => {
    void props.initiativeId;
    void act(reload);
    const timer = setInterval(() => {
      if (jobs().some(isUnfinished)) void reload();
    }, POLL_MS);
    onCleanup(() => clearInterval(timer));
  });

  const listed = () => jobs().filter((j) => (j.state === JobState.JobCancelled) === (tab() === "cancelled"));
  const cancelledCount = () => jobs().filter((j) => j.state === JobState.JobCancelled).length;

  const startDemo = () =>
    act(() =>
      JobService.Enqueue({
        kind: "demo",
        initiativeId: props.initiativeId,
        params: JSON.stringify({ delayMs: 500, failAt: -1 }),
        totalItems: 4,
      }),
    );

  return (
    <section class="record-section jobs" aria-label="Jobs">
      <h3>Jobs</h3>

      <div class="area-tabs" role="tablist" aria-label="Job lists">
        <button
          class="area-tab"
          classList={{ active: tab() === "active" }}
          role="tab"
          aria-selected={tab() === "active"}
          onClick={() => setTab("active")}
        >
          Current
        </button>
        <button
          class="area-tab"
          classList={{ active: tab() === "cancelled" }}
          role="tab"
          aria-selected={tab() === "cancelled"}
          onClick={() => setTab("cancelled")}
        >
          Cancelled ({cancelledCount()})
        </button>
      </div>

      <ul class="record-list" role="tabpanel" aria-label={tab() === "cancelled" ? "Cancelled jobs" : "Current jobs"}>
        <For
          each={listed()}
          fallback={<li class="muted">{tab() === "cancelled" ? "Nothing cancelled." : "No jobs yet."}</li>}
        >
          {(job) => (
            <li class="job-row">
              <span class="job-name">
                {job.kind}
                <span class="muted">
                  {" "}
                  — {job.state}, {job.completedItems}/{job.totalItems}
                  <Show when={job.failureReason}> ({job.failureReason})</Show>
                </span>
              </span>
              <Show when={isUnfinished(job)}>
                <button aria-label={`Cancel job ${job.id}`} onClick={() => act(() => JobService.Cancel(job.id))}>
                  Cancel
                </button>
              </Show>
              <Show when={job.state === JobState.JobFailed || job.state === JobState.JobCancelled}>
                <button aria-label={`Retry job ${job.id}`} onClick={() => act(() => JobService.Retry(job.id))}>
                  Retry
                </button>
              </Show>
            </li>
          )}
        </For>
      </ul>

      {/* The demo job is the only worker this phase registers: the real
          pipelines arrive with the phases that need them. */}
      <div class="record-form-actions">
        <button onClick={startDemo}>Start demo job</button>
      </div>

      <Show when={error()}>
        <p class="modal-error" role="alert">{error()}</p>
      </Show>
    </section>
  );
}
