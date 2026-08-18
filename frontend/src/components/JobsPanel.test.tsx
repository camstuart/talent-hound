import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@solidjs/testing-library";
import JobsPanel from "./JobsPanel";

// The Go backend is not running: bindings are mocked. Fixtures are invented.
const { state, mocks } = vi.hoisted(() => {
  const state = {
    jobs: [] as Record<string, unknown>[],
    error: "",
  };
  return {
    state,
    mocks: {
      ListForInitiative: vi.fn(async () => state.jobs),
      Enqueue: vi.fn(async (input: Record<string, unknown>) => {
        if (state.error) throw new Error(state.error);
        return { id: 9, ...input };
      }),
      Cancel: vi.fn(async () => undefined),
      Retry: vi.fn(async () => ({ id: 1 })),
    },
  };
});

vi.mock("../../bindings/camstuart/talent-hound", () => ({ JobService: mocks }));

const aJob = (over: Record<string, unknown> = {}) => ({
  id: 1,
  kind: "demo",
  state: "running",
  totalItems: 4,
  completedItems: 1,
  failureReason: "",
  ...over,
});

beforeEach(() => {
  state.jobs = [];
  state.error = "";
  Object.values(mocks).forEach((m) => m.mockClear());
});

describe("JobsPanel", () => {
  it("shows a running job's state and progress", async () => {
    state.jobs = [aJob()];
    render(() => <JobsPanel initiativeId={7} />);

    await waitFor(() => expect(screen.getByText(/running, 1\/4/)).toBeInTheDocument());
    expect(mocks.ListForInitiative).toHaveBeenCalledWith(7);
  });

  it("says so when there are no jobs", async () => {
    render(() => <JobsPanel initiativeId={7} />);

    await waitFor(() => expect(screen.getByText("No jobs yet.")).toBeInTheDocument());
  });

  it("cancels an unfinished job", async () => {
    state.jobs = [aJob()];
    render(() => <JobsPanel initiativeId={7} />);
    await waitFor(() => expect(screen.getByLabelText("Cancel job 1")).toBeInTheDocument());

    fireEvent.click(screen.getByLabelText("Cancel job 1"));
    await waitFor(() => expect(mocks.Cancel).toHaveBeenCalledWith(1));
  });

  it("shows a failed job's reason in the main list and offers retry", async () => {
    state.jobs = [aJob({ state: "failed", failureReason: "interrupted", completedItems: 2 })];
    render(() => <JobsPanel initiativeId={7} />);

    // A failure is news: it stays where the recruiter is looking.
    await waitFor(() => expect(screen.getByText(/failed, 2\/4 \(interrupted\)/)).toBeInTheDocument());
    expect(screen.queryByLabelText("Cancel job 1")).not.toBeInTheDocument();

    fireEvent.click(screen.getByLabelText("Retry job 1"));
    await waitFor(() => expect(mocks.Retry).toHaveBeenCalledWith(1));
  });

  it("keeps cancelled jobs in their own tab, with a count", async () => {
    state.jobs = [aJob(), aJob({ id: 2, state: "cancelled", completedItems: 3 })];
    render(() => <JobsPanel initiativeId={7} />);

    // The active list holds the job the recruiter is watching, and nothing else.
    await waitFor(() => expect(screen.getByText(/running, 1\/4/)).toBeInTheDocument());
    expect(screen.queryByText(/cancelled, 3\/4/)).not.toBeInTheDocument();
    expect(screen.getByText("Cancelled (1)")).toBeInTheDocument();

    fireEvent.click(screen.getByText("Cancelled (1)"));
    await waitFor(() => expect(screen.getByText(/cancelled, 3\/4/)).toBeInTheDocument());
    expect(screen.queryByText(/running, 1\/4/)).not.toBeInTheDocument();
    // A cancelled job can still be retried.
    expect(screen.getByLabelText("Retry job 2")).toBeInTheDocument();
  });

  it("starts a demo job against this initiative", async () => {
    render(() => <JobsPanel initiativeId={7} />);
    await waitFor(() => expect(mocks.ListForInitiative).toHaveBeenCalled());

    fireEvent.click(screen.getByText("Start demo job"));
    await waitFor(() => expect(mocks.Enqueue).toHaveBeenCalled());
    const input = mocks.Enqueue.mock.calls[0][0] as Record<string, unknown>;
    expect(input.kind).toBe("demo");
    expect(input.initiativeId).toBe(7);
  });

  it("shows the backend's own words when a request is refused", async () => {
    state.error = "no worker registered for job kind \"demo\"";
    render(() => <JobsPanel initiativeId={7} />);
    await waitFor(() => expect(mocks.ListForInitiative).toHaveBeenCalled());

    fireEvent.click(screen.getByText("Start demo job"));

    expect(await screen.findByText(state.error)).toBeInTheDocument();
  });
});
