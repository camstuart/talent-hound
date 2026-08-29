package main

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"gorm.io/gorm"

	"camstuart/talent-hound/internal/db"
	"camstuart/talent-hound/internal/models"
)

// Every fixture here is invented. No real candidate information enters this
// repository, its logs, or its test output.

// newJobDB uses a file database rather than ":memory:": jobs run on their own
// goroutines, and a second connection to an in-memory SQLite database is a
// second, empty database.
func newJobDB(t *testing.T) *gorm.DB {
	t.Helper()
	gdb, err := db.Open(filepath.Join(t.TempDir(), "jobs.db"))
	if err != nil {
		t.Fatalf("opening db: %v", err)
	}
	closeOnCleanup(t, gdb)
	return gdb
}

func newJobService(t *testing.T) (*JobService, *gorm.DB) {
	t.Helper()
	gdb := newJobDB(t)
	return NewJobService(gdb), gdb
}

// waitForState polls until the job reaches one of want, or the test gives up.
// Polling is the honest way to observe a lifecycle whose whole point is that it
// lives in the database rather than in a channel.
func waitForState(t *testing.T, s *JobService, id uint, want ...models.JobState) *models.Job {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	var last models.JobState
	for time.Now().Before(deadline) {
		job, err := s.Get(id)
		if err != nil {
			t.Fatalf("loading job %d: %v", id, err)
		}
		last = job.State
		for _, w := range want {
			if job.State == w {
				return job
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("job %d is %s, waited for one of %v", id, last, want)
	return nil
}

// waitForCompleted polls until the job's committed item count reaches n.
func waitForCompleted(t *testing.T, s *JobService, id uint, n int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		job, err := s.Get(id)
		if err != nil {
			t.Fatalf("loading job %d: %v", id, err)
		}
		if job.CompletedItems >= n {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("job %d never reached %d completed items", id, n)
}

// countCompanies is the stand-in for "work a worker committed". Companies are
// the simplest row an item can write, and nothing else in these tests makes one.
func countCompanies(t *testing.T, gdb *gorm.DB) int64 {
	t.Helper()
	var n int64
	if err := gdb.Model(&models.Company{}).Count(&n).Error; err != nil {
		t.Fatalf("counting companies: %v", err)
	}
	return n
}

// insertJob writes a job row directly, so a test can start from any state
// without racing a runner into it.
func insertJob(t *testing.T, gdb *gorm.DB, state models.JobState, total, completed int) uint {
	t.Helper()
	job := &models.Job{Kind: "demo", Params: "{}", State: state, TotalItems: total, CompletedItems: completed}
	if err := gdb.Create(job).Error; err != nil {
		t.Fatalf("inserting %s job: %v", state, err)
	}
	return job.ID
}

func TestEveryIllegalTransitionIsRefused(t *testing.T) {
	svc, gdb := newJobService(t)

	legal := map[string]bool{
		"queued>running":    true,
		"queued>cancelled":  true,
		"running>completed": true,
		"running>failed":    true,
		"running>cancelled": true,
	}

	for _, from := range models.JobStates {
		for _, to := range models.JobStates {
			pair := fmt.Sprintf("%s>%s", from, to)
			id := insertJob(t, gdb, from, 1, 0)
			moved, err := svc.transition(id, from, to, nil)

			if legal[pair] {
				if err != nil || !moved {
					t.Errorf("%s should be allowed: moved=%v err=%v", pair, moved, err)
				}
				continue
			}
			if err == nil {
				t.Errorf("%s should be refused", pair)
			}
			job, gerr := svc.Get(id)
			if gerr != nil {
				t.Fatalf("loading job: %v", gerr)
			}
			if job.State != from {
				t.Errorf("%s left the job in %s, want it unchanged at %s", pair, job.State, from)
			}
		}
	}
}

func TestTerminalStatesHaveNoSuccessor(t *testing.T) {
	for _, s := range []models.JobState{models.JobCompleted, models.JobFailed, models.JobCancelled} {
		if !s.Final() {
			t.Errorf("%s should be final", s)
		}
		for _, to := range models.JobStates {
			if s.CanTransition(to) {
				t.Errorf("%s should not be able to move to %s", s, to)
			}
		}
	}
}

func TestEnqueueRefusesUnknownKind(t *testing.T) {
	svc, gdb := newJobService(t)

	if _, err := svc.Enqueue(JobInput{Kind: "not_a_kind", TotalItems: 1}); err == nil {
		t.Fatal("enqueuing an unregistered kind should fail")
	}
	var n int64
	if err := gdb.Model(&models.Job{}).Count(&n).Error; err != nil {
		t.Fatalf("counting jobs: %v", err)
	}
	if n != 0 {
		t.Errorf("a refused enqueue persisted %d jobs", n)
	}
}

func TestZeroItemJobCompletes(t *testing.T) {
	svc, _ := newJobService(t)

	job, err := svc.Enqueue(JobInput{Kind: "demo", TotalItems: 0})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	done := waitForState(t, svc, job.ID, models.JobCompleted)
	if done.CompletedItems != 0 || done.TotalItems != 0 {
		t.Errorf("empty batch finished at %d/%d, want 0/0", done.CompletedItems, done.TotalItems)
	}
}

func TestProgressCountsCommittedItems(t *testing.T) {
	svc, gdb := newJobService(t)
	svc.register("writer", writerWorker)

	job, err := svc.Enqueue(JobInput{Kind: "writer", TotalItems: 3})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	done := waitForState(t, svc, job.ID, models.JobCompleted)
	if done.CompletedItems != 3 {
		t.Errorf("completed %d items, want 3", done.CompletedItems)
	}
	if got := countCompanies(t, gdb); got != 3 {
		t.Errorf("%d rows committed, want 3", got)
	}
}

func TestCancelBeforeStart(t *testing.T) {
	svc, gdb := newJobService(t)

	// Inserted rather than enqueued: a queued job that no runner has picked up
	// is exactly the state this scenario is about.
	id := insertJob(t, gdb, models.JobQueued, 3, 0)
	if err := svc.Cancel(id); err != nil {
		t.Fatalf("cancelling a queued job: %v", err)
	}
	job, err := svc.Get(id)
	if err != nil {
		t.Fatalf("loading job: %v", err)
	}
	if job.State != models.JobCancelled || job.CompletedItems != 0 {
		t.Errorf("job is %s at %d items, want cancelled at 0", job.State, job.CompletedItems)
	}
	if got := countCompanies(t, gdb); got != 0 {
		t.Errorf("a job cancelled before start committed %d rows", got)
	}
}

func TestCancelDuringAnItemRollsItBack(t *testing.T) {
	svc, gdb := newJobService(t)
	entered := make(chan int, 8)
	release := make(chan struct{}, 8)
	svc.register("gated", gatedWorker(entered, release))

	job, err := svc.Enqueue(JobInput{Kind: "gated", TotalItems: 1})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	<-entered // the item is in flight, its row already written inside the tx

	if err := svc.Cancel(job.ID); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	done := waitForState(t, svc, job.ID, models.JobCancelled)
	if done.CompletedItems != 0 {
		t.Errorf("completed count is %d, want 0: the item rolled back", done.CompletedItems)
	}
	if got := countCompanies(t, gdb); got != 0 {
		t.Errorf("the interrupted item left %d rows behind", got)
	}
}

func TestCancelKeepsCompletedItems(t *testing.T) {
	svc, gdb := newJobService(t)
	entered := make(chan int, 8)
	release := make(chan struct{}, 8)
	svc.register("gated", gatedWorker(entered, release))

	job, err := svc.Enqueue(JobInput{Kind: "gated", TotalItems: 3})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	<-entered
	release <- struct{}{} // exactly one item gets through
	waitForCompleted(t, svc, job.ID, 1)

	if err := svc.Cancel(job.ID); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	done := waitForState(t, svc, job.ID, models.JobCancelled)
	if done.CompletedItems < 1 || done.CompletedItems >= done.TotalItems {
		t.Fatalf("cancelled at %d/%d, want a partial count", done.CompletedItems, done.TotalItems)
	}
	// The count and the committed work agree exactly: that is what the per-item
	// transaction boundary buys.
	if got := countCompanies(t, gdb); got != int64(done.CompletedItems) {
		t.Errorf("%d rows committed but the job claims %d items", got, done.CompletedItems)
	}
}

func TestCancelAfterCompletionIsRefused(t *testing.T) {
	svc, _ := newJobService(t)

	job, err := svc.Enqueue(JobInput{Kind: "demo", TotalItems: 0})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	waitForState(t, svc, job.ID, models.JobCompleted)

	if err := svc.Cancel(job.ID); err == nil {
		t.Fatal("cancelling a completed job should be refused")
	}
	after, err := svc.Get(job.ID)
	if err != nil {
		t.Fatalf("loading job: %v", err)
	}
	if after.State != models.JobCompleted {
		t.Errorf("a refused cancel left the job %s", after.State)
	}
}

func TestCancelIsIdempotent(t *testing.T) {
	svc, gdb := newJobService(t)

	id := insertJob(t, gdb, models.JobQueued, 1, 0)
	for i := range 2 {
		if err := svc.Cancel(id); err != nil {
			t.Fatalf("cancel %d: %v", i+1, err)
		}
	}
	job, err := svc.Get(id)
	if err != nil {
		t.Fatalf("loading job: %v", err)
	}
	if job.State != models.JobCancelled {
		t.Errorf("job is %s after two cancels, want cancelled", job.State)
	}
}

func TestRetryRunsAgainAndClearsTheFailure(t *testing.T) {
	svc, _ := newJobService(t)
	attempts := 0
	svc.register("flaky", func(_ context.Context, _ models.Job, item int) (JobCommit, error) {
		if attempts == 0 && item == 0 {
			attempts++
			return nil, FailReason("demo_failure")
		}
		return nil, nil
	})

	job, err := svc.Enqueue(JobInput{Kind: "flaky", TotalItems: 2})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	failed := waitForState(t, svc, job.ID, models.JobFailed)
	if failed.FailureReason != "demo_failure" || failed.CompletedItems != 0 {
		t.Fatalf("failed with %q at %d items", failed.FailureReason, failed.CompletedItems)
	}

	if err := svc.Retry(job.ID); err != nil {
		t.Fatalf("retry: %v", err)
	}
	done := waitForState(t, svc, job.ID, models.JobCompleted)
	if done.FailureReason != "" || done.CompletedItems != 2 {
		t.Errorf("after retry: reason %q at %d/2 items", done.FailureReason, done.CompletedItems)
	}
}

func TestRetryClearsAPendingCancellation(t *testing.T) {
	svc, _ := newJobService(t)
	entered := make(chan int, 8)
	release := make(chan struct{}, 8)
	svc.register("gated", gatedWorker(entered, release))

	job, err := svc.Enqueue(JobInput{Kind: "gated", TotalItems: 2})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	<-entered
	if err := svc.Cancel(job.ID); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	waitForState(t, svc, job.ID, models.JobCancelled)

	if err := svc.Retry(job.ID); err != nil {
		t.Fatalf("retry: %v", err)
	}
	// The earlier request must not cancel the new run: both items get through.
	release <- struct{}{}
	release <- struct{}{}
	done := waitForState(t, svc, job.ID, models.JobCompleted)
	if done.CompletedItems != 2 || done.FailureReason != "" {
		t.Errorf("the retried run finished at %d/2 with %q", done.CompletedItems, done.FailureReason)
	}
}

func TestRetryIsRefusedUntilTheJobStops(t *testing.T) {
	svc, gdb := newJobService(t)

	for _, state := range []models.JobState{models.JobQueued, models.JobRunning, models.JobCompleted} {
		id := insertJob(t, gdb, state, 1, 0)
		if err := svc.Retry(id); err == nil {
			t.Errorf("retrying a %s job should be refused", state)
		}
	}
}

func TestRepeatedRetryLeavesOneRun(t *testing.T) {
	svc, gdb := newJobService(t)
	entered := make(chan int, 8)
	release := make(chan struct{}, 8)
	svc.register("gated", gatedWorker(entered, release))

	id := insertJob(t, gdb, models.JobFailed, 1, 0)
	if err := gdb.Model(&models.Job{}).Where("id = ?", id).Update("kind", "gated").Error; err != nil {
		t.Fatalf("setting kind: %v", err)
	}

	if err := svc.Retry(id); err != nil {
		t.Fatalf("first retry: %v", err)
	}
	// The second request arrives against a job that has started again, so it is
	// refused rather than queuing a duplicate run.
	if err := svc.Retry(id); err == nil {
		t.Error("a second retry should be refused while the job is running")
	}
	<-entered
	release <- struct{}{}
	done := waitForState(t, svc, id, models.JobCompleted)
	if done.CompletedItems != 1 {
		t.Errorf("two retries produced %d completed items, want 1", done.CompletedItems)
	}
}

func TestRestartFailsUnfinishedJobsExactlyOnce(t *testing.T) {
	gdb := newJobDB(t)
	first := NewJobService(gdb)

	running := insertJob(t, gdb, models.JobRunning, 5, 2)
	queued := insertJob(t, gdb, models.JobQueued, 5, 0)
	completed := insertJob(t, gdb, models.JobCompleted, 5, 5)
	_ = first

	// A second service on the same database is a restart: the process that held
	// those jobs is gone.
	second := NewJobService(gdb)
	for _, id := range []uint{running, queued} {
		job, err := second.Get(id)
		if err != nil {
			t.Fatalf("loading job: %v", err)
		}
		if job.State != models.JobFailed || job.FailureReason != models.ReasonInterrupted {
			t.Errorf("job %d is %s/%q, want failed/interrupted", id, job.State, job.FailureReason)
		}
	}
	if job, _ := second.Get(running); job.CompletedItems != 2 {
		t.Errorf("recovery changed the completed count to %d, want 2", job.CompletedItems)
	}
	if job, _ := second.Get(completed); job.State != models.JobCompleted || job.FailureReason != "" {
		t.Errorf("recovery touched a finished job: %s/%q", job.State, job.FailureReason)
	}

	// Starting again changes nothing further.
	before, _ := second.Get(running)
	third := NewJobService(gdb)
	after, _ := third.Get(running)
	if after.State != before.State || !after.UpdatedAt.Equal(before.UpdatedAt) {
		t.Errorf("a second start-up changed job %d again", running)
	}

	// And an interrupted job is retryable.
	if err := third.Retry(running); err != nil {
		t.Fatalf("retrying an interrupted job: %v", err)
	}
	waitForState(t, third, running, models.JobCompleted)
}

func TestPanicFailsTheJobNotTheApplication(t *testing.T) {
	svc, gdb := newJobService(t)
	// It panics in the commit half, so the test also proves the item's write is
	// rolled back rather than merely never attempted.
	svc.register("panicky", func(_ context.Context, _ models.Job, _ int) (JobCommit, error) {
		return func(tx *gorm.DB) error {
			if err := tx.Create(&models.Company{Name: "Never committed"}).Error; err != nil {
				return err
			}
			panic("worker exploded")
		}, nil
	})

	job, err := svc.Enqueue(JobInput{Kind: "panicky", TotalItems: 1})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	failed := waitForState(t, svc, job.ID, models.JobFailed)
	if failed.FailureReason != models.ReasonPanic {
		t.Errorf("reason is %q, want %q", failed.FailureReason, models.ReasonPanic)
	}
	if got := countCompanies(t, gdb); got != 0 {
		t.Errorf("the panicking item left %d rows behind", got)
	}

	// The service still works.
	next, err := svc.Enqueue(JobInput{Kind: "demo", TotalItems: 0})
	if err != nil {
		t.Fatalf("enqueue after a panic: %v", err)
	}
	waitForState(t, svc, next.ID, models.JobCompleted)
}

func TestUndeclaredErrorsAreStoredAsACode(t *testing.T) {
	svc, _ := newJobService(t)
	// The error text is exactly the kind of thing that must never be stored.
	svc.register("leaky", func(_ context.Context, _ models.Job, _ int) (JobCommit, error) {
		return nil, errors.New("no resume found for Priya Raman <priya@example.invalid>")
	})

	job, err := svc.Enqueue(JobInput{Kind: "leaky", TotalItems: 1})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	failed := waitForState(t, svc, job.ID, models.JobFailed)
	if failed.FailureReason != models.ReasonWorkerError {
		t.Errorf("reason is %q, want %q", failed.FailureReason, models.ReasonWorkerError)
	}
}

func TestFailureKeepsTheItemsThatCommitted(t *testing.T) {
	svc, gdb := newJobService(t)
	svc.register("writer", func(ctx context.Context, job models.Job, item int) (JobCommit, error) {
		if item == 2 {
			return nil, FailReason("demo_failure")
		}
		return writerWorker(ctx, job, item)
	})

	job, err := svc.Enqueue(JobInput{Kind: "writer", TotalItems: 4})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	failed := waitForState(t, svc, job.ID, models.JobFailed)
	if failed.CompletedItems != 2 {
		t.Errorf("completed count is %d, want 2", failed.CompletedItems)
	}
	if got := countCompanies(t, gdb); got != 2 {
		t.Errorf("%d rows survived the failure, want 2", got)
	}
}

func TestFailureReasonRejectsFreeText(t *testing.T) {
	svc, gdb := newJobService(t)

	for _, bad := range []string{"Priya Raman was not found", "Timeout!", "UPPER", "", "a-dash"} {
		if models.ValidReason(bad) {
			t.Errorf("%q should not be a valid reason code", bad)
		}
	}
	for _, good := range []string{"interrupted", "worker_error", "panic", "extraction_failed", "e1"} {
		if !models.ValidReason(good) {
			t.Errorf("%q should be a valid reason code", good)
		}
	}

	id := insertJob(t, gdb, models.JobRunning, 1, 0)
	if _, err := svc.transition(id, models.JobRunning, models.JobFailed, map[string]any{
		"failure_reason": "the candidate's file was missing",
	}); err == nil {
		t.Fatal("a sentence should not be storable as a failure reason")
	}
	job, err := svc.Get(id)
	if err != nil {
		t.Fatalf("loading job: %v", err)
	}
	// And the database refuses it too, not only the service: a sentence must
	// not be storable by any route.
	if err := gdb.Model(&models.Job{}).Where("id = ?", id).
		UpdateColumn("failure_reason", "the candidate's file was missing").Error; err == nil {
		t.Error("the database accepted a sentence as a failure reason")
	}
	if job.State != models.JobRunning || job.FailureReason != "" {
		t.Errorf("the refused write left %s/%q", job.State, job.FailureReason)
	}
}

func TestListForInitiativeIncludesUnattachedJobs(t *testing.T) {
	svc, gdb := newJobService(t)
	mine := anInitiative(t, gdb, "Jobs list mine")
	other := anInitiative(t, gdb, "Jobs list other")

	for _, in := range []JobInput{
		{Kind: "demo", TotalItems: 0, InitiativeID: mine},
		{Kind: "demo", TotalItems: 0, InitiativeID: other},
		{Kind: "demo", TotalItems: 0},
	} {
		if _, err := svc.Enqueue(in); err != nil {
			t.Fatalf("enqueue: %v", err)
		}
	}

	jobs, err := svc.ListForInitiative(mine)
	if err != nil {
		t.Fatalf("listing: %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("listed %d jobs, want this initiative's plus the unattached one", len(jobs))
	}
	if _, err := svc.Enqueue(JobInput{Kind: "demo", InitiativeID: 9999}); err == nil {
		t.Error("enqueuing against a missing initiative should fail")
	}
}

// writerWorker commits one row per item, so a test can compare what a job
// claims it finished against what is actually in the database.
func writerWorker(_ context.Context, _ models.Job, item int) (JobCommit, error) {
	return func(tx *gorm.DB) error {
		return tx.Create(&models.Company{Name: fmt.Sprintf("Item %d", item)}).Error
	}, nil
}

// gatedWorker announces itself and then waits — for the test to hand it a
// token, or for cancellation — before committing its row. One token lets one
// item through, so a test decides exactly how far a job gets.
func gatedWorker(entered chan<- int, release <-chan struct{}) JobWorker {
	return func(ctx context.Context, job models.Job, item int) (JobCommit, error) {
		entered <- item
		select {
		case <-release:
			return writerWorker(ctx, job, item)
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}
