package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"gorm.io/gorm"

	"camstuart/talent-hound/internal/models"
)

// JobService runs slow work with one lifecycle: queued, running, then finished
// one of three ways. Nothing here knows what the work is — the pipelines
// arriving in later phases register a worker per kind and get progress,
// cancellation, retry, and honest post-crash state for free.
//
// Each item runs in its own transaction together with the increment of the
// completed count, so a cancelled batch keeps every item that committed and
// nothing from the one that did not.
type JobService struct {
	db      *gorm.DB
	workers map[string]JobWorker

	mu      sync.Mutex
	cancels map[uint]context.CancelFunc
	// cancelling holds the jobs whose cancellation has been requested but whose
	// runner has not stopped yet. It is memory rather than a column because
	// SQLite has one writer: while an item's transaction is open, no other
	// connection can write, so a cancellation that needed a write would have to
	// wait for the very item it is trying to interrupt. Nothing is lost — a
	// request that does not outlive the process is moot, because a restart
	// fails the job as interrupted anyway.
	cancelling map[uint]bool
	// running is held for the lifetime of every job goroutine, so tests (and a
	// future shutdown path) can wait for them instead of sleeping.
	running sync.WaitGroup
}

// JobWorker does one item's work with no transaction open, and returns the
// closure that writes the result. The runner calls that closure inside the same
// transaction that records the item as completed, so the work and the count
// still commit together — but the database's single writer is held for the
// length of one update rather than the length of the work.
//
// A worker with nothing to write returns a nil JobCommit. A worker that wants a
// specific failure reason returns FailReason, from either half.
type JobWorker func(ctx context.Context, job models.Job, item int) (JobCommit, error)

// JobCommit writes one item's result. Everything it touches must go through tx.
type JobCommit func(tx *gorm.DB) error

// NewJobService returns a JobService backed by db, with the demo worker
// registered, after recovering jobs left unfinished by a previous run.
//
// The sweep lives here rather than in main so it happens exactly once per
// process, before any binding can start a job, and so the server build used by
// the E2E tests gets it without a second call site to forget.
func NewJobService(db *gorm.DB) *JobService {
	s := &JobService{
		db:         db,
		workers:    map[string]JobWorker{},
		cancels:    map[uint]context.CancelFunc{},
		cancelling: map[uint]bool{},
	}
	s.register("demo", demoWorker)
	if err := s.sweepInterrupted(); err != nil {
		// A database that cannot record the sweep is one that cannot run jobs
		// either, so the next enqueue will fail loudly; nothing is hidden by
		// logging this and carrying on.
		log.Println("job recovery failed:", err)
	}
	return s
}

// register adds a worker for one job kind. Kinds are code, not data: enqueuing
// an unregistered kind fails immediately rather than creating a job that
// nothing will ever run.
func (s *JobService) register(kind string, w JobWorker) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.workers[kind] = w
}

// sweepInterrupted fails every job left queued or running by a previous
// process. The PRD names running jobs; a queued one is in the same position,
// because the queue lived in the process that is gone.
func (s *JobService) sweepInterrupted() error {
	now := time.Now().UTC()
	err := s.db.Model(&models.Job{}).
		Where("state IN ?", []models.JobState{models.JobQueued, models.JobRunning}).
		Updates(map[string]any{
			"state":          models.JobFailed,
			"failure_reason": models.ReasonInterrupted,
			"finished_at":    now,
		}).Error
	if err != nil {
		return fmt.Errorf("recovering interrupted jobs: %w", err)
	}
	return nil
}

// JobInput is a request to run something.
type JobInput struct {
	Kind string `json:"kind"`
	// The initiative whose workspace shows this job; zero for work that belongs
	// to no single initiative.
	InitiativeID uint `json:"initiativeId"`
	// Worker settings as JSON: identifiers and numbers, never content.
	Params     string `json:"params"`
	TotalItems int    `json:"totalItems"`
}

// Enqueue persists a job and starts it. A job with no items is legitimate — an
// empty batch is a batch that finished — and completes without running anything.
func (s *JobService) Enqueue(in JobInput) (*models.Job, error) {
	s.mu.Lock()
	_, known := s.workers[in.Kind]
	s.mu.Unlock()
	if !known {
		return nil, fmt.Errorf("no worker registered for job kind %q", in.Kind)
	}
	if in.TotalItems < 0 {
		return nil, fmt.Errorf("job total items must not be negative, got %d", in.TotalItems)
	}
	if in.Params == "" {
		in.Params = "{}"
	}
	if !json.Valid([]byte(in.Params)) {
		return nil, fmt.Errorf("job params must be JSON")
	}

	job := &models.Job{
		Kind:       in.Kind,
		Params:     in.Params,
		State:      models.JobQueued,
		TotalItems: in.TotalItems,
	}
	if in.InitiativeID != 0 {
		id := in.InitiativeID
		var n int64
		if err := s.db.Model(&models.Initiative{}).Where("id = ?", id).Count(&n).Error; err != nil {
			return nil, fmt.Errorf("checking initiative %d: %w", id, err)
		}
		if n == 0 {
			return nil, fmt.Errorf("initiative %d does not exist", id)
		}
		job.InitiativeID = &id
	}
	if err := s.db.Create(job).Error; err != nil {
		return nil, fmt.Errorf("creating job: %w", err)
	}
	s.start(job.ID)
	return job, nil
}

// Get returns one job.
func (s *JobService) Get(id uint) (*models.Job, error) {
	var job models.Job
	if err := s.db.First(&job, id).Error; err != nil {
		return nil, fmt.Errorf("loading job %d: %w", id, err)
	}
	return &job, nil
}

// List returns every job, newest first.
func (s *JobService) List() ([]models.Job, error) {
	return s.list(s.db.Order("id desc"))
}

// ListForInitiative returns one initiative's jobs plus the jobs that belong to
// no initiative, newest first.
func (s *JobService) ListForInitiative(initiativeID uint) ([]models.Job, error) {
	return s.list(s.db.Where("initiative_id = ? OR initiative_id IS NULL", initiativeID).Order("id desc"))
}

func (s *JobService) list(q *gorm.DB) ([]models.Job, error) {
	jobs := []models.Job{}
	if err := q.Find(&jobs).Error; err != nil {
		return nil, fmt.Errorf("listing jobs: %w", err)
	}
	return jobs, nil
}

// Cancel stops a job that has not finished. A queued job stops at once; a
// running one stops at the next item boundary, and the item in flight rolls
// back. Cancelling an already-cancelled job is not an error: the caller asked
// for a state that already holds.
func (s *JobService) Cancel(id uint) error {
	job, err := s.Get(id)
	if err != nil {
		return err
	}
	switch job.State {
	case models.JobCancelled:
		return nil
	case models.JobCompleted, models.JobFailed:
		return fmt.Errorf("job %d has already %s", id, job.State)
	case models.JobQueued:
		// The runner may be starting it right now; the guarded update decides.
		moved, err := s.transition(id, models.JobQueued, models.JobCancelled, map[string]any{
			"finished_at": time.Now().UTC(),
		})
		if err != nil {
			return err
		}
		if moved {
			return nil
		}
	case models.JobRunning:
	}

	// Running, or queued-but-just-started. The request is recorded in memory and
	// the context is cancelled, so an item that is waiting stops at once; the
	// runner turns that into the durable Cancelled state at the item boundary.
	s.mu.Lock()
	s.cancelling[id] = true
	cancel := s.cancels[id]
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	return nil
}

// Retry runs a stopped job again from the start. Retrying a job that has not
// stopped is refused: there is nothing to retry while it is still going.
func (s *JobService) Retry(id uint) error {
	job, err := s.Get(id)
	if err != nil {
		return err
	}
	switch job.State {
	case models.JobFailed, models.JobCancelled:
	case models.JobCompleted:
		return fmt.Errorf("job %d completed, so there is nothing to retry", id)
	default:
		return fmt.Errorf("job %d is %s, so it cannot be retried yet", id, job.State)
	}

	// A cancellation left over from the run that stopped must not cancel this one.
	s.mu.Lock()
	delete(s.cancelling, id)
	s.mu.Unlock()

	// Guarded on the state we read, so two retries cannot both queue a run.
	res := s.db.Model(&models.Job{}).Where("id = ? AND state = ?", id, job.State).
		Updates(map[string]any{
			"state":           models.JobQueued,
			"completed_items": 0,
			"failure_reason":  "",
			"started_at":      nil,
			"finished_at":     nil,
		})
	if res.Error != nil {
		return fmt.Errorf("retrying job %d: %w", id, res.Error)
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("job %d changed state while being retried", id)
	}
	s.start(id)
	return nil
}

// transition is the only place a job's state changes. The guard on the current
// state makes the change a compare-and-set, so a cancel racing a start cannot
// produce a job that is both.
func (s *JobService) transition(id uint, from, to models.JobState, extra map[string]any) (bool, error) {
	if !from.CanTransition(to) {
		return false, fmt.Errorf("job %d cannot go from %s to %s", id, from, to)
	}
	updates := map[string]any{"state": to}
	for k, v := range extra {
		updates[k] = v
	}
	if reason, ok := updates["failure_reason"].(string); ok && reason != "" {
		if err := models.CheckReason(reason); err != nil {
			return false, err
		}
	}
	res := s.db.Model(&models.Job{}).Where("id = ? AND state = ?", id, from).Updates(updates)
	if res.Error != nil {
		return false, fmt.Errorf("moving job %d to %s: %w", id, to, res.Error)
	}
	return res.RowsAffected == 1, nil
}

// start runs a job in its own goroutine.
//
// ponytail: one goroutine per job, no pool. This is a single-user desktop app
// where the slow thing is a model call the recruiter is watching; add a
// semaphore when two pipelines can realistically contend for the same CPU.
func (s *JobService) start(id uint) {
	s.running.Add(1)
	go func() {
		defer s.running.Done()
		s.run(id)
	}()
}

// run drives one job through its items. Every exit path leaves the job in a
// final state: a job left running is a lie the next start-up has to clean up.
func (s *JobService) run(id uint) {
	ctx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	s.cancels[id] = cancel
	s.mu.Unlock()
	defer func() {
		cancel()
		s.mu.Lock()
		delete(s.cancels, id)
		delete(s.cancelling, id)
		s.mu.Unlock()
	}()

	job, err := s.Get(id)
	if err != nil {
		return
	}
	s.mu.Lock()
	worker := s.workers[job.Kind]
	s.mu.Unlock()
	if worker == nil {
		s.finish(id, models.JobFailed, models.ReasonWorkerError)
		return
	}

	// Cancelled while queued: the guard fails and there is nothing to undo.
	started, err := s.transition(id, models.JobQueued, models.JobRunning, map[string]any{
		"started_at": time.Now().UTC(),
	})
	if err != nil || !started {
		return
	}

	for item := 0; item < job.TotalItems; item++ {
		if s.cancelRequested(id) {
			s.finish(id, models.JobCancelled, "")
			return
		}
		// The work happens first, with nothing locked. Only its result enters a
		// transaction — together with the count that claims it, so the two
		// commit together or neither does.
		commit, err := safeWork(func() (JobCommit, error) { return worker(ctx, *job, item) })
		if err == nil {
			err = s.db.Transaction(func(tx *gorm.DB) error {
				if commit != nil {
					if err := safeCall(func() error { return commit(tx) }); err != nil {
						return err
					}
				}
				return tx.Model(&models.Job{}).Where("id = ?", id).
					UpdateColumn("completed_items", gorm.Expr("completed_items + 1")).Error
			})
		}
		if err == nil {
			continue
		}
		// A failure while cancellation was pending is the cancellation: the
		// item rolled back, which is exactly what cancelling should do.
		if s.cancelRequested(id) {
			s.finish(id, models.JobCancelled, "")
			return
		}
		s.finish(id, models.JobFailed, reasonOf(err))
		return
	}

	if s.cancelRequested(id) {
		s.finish(id, models.JobCancelled, "")
		return
	}
	s.finish(id, models.JobCompleted, "")
}

// finish moves a running job to its final state, recording when it stopped.
func (s *JobService) finish(id uint, state models.JobState, reason string) {
	extra := map[string]any{"finished_at": time.Now().UTC()}
	if reason != "" {
		extra["failure_reason"] = reason
	}
	if _, err := s.transition(id, models.JobRunning, state, extra); err != nil {
		// The only way here is a reason that is not a code, which is a bug in
		// the worker's declaration rather than something the recruiter can fix.
		_, _ = s.transition(id, models.JobRunning, models.JobFailed, map[string]any{
			"finished_at":    time.Now().UTC(),
			"failure_reason": models.ReasonWorkerError,
		})
	}
}

// cancelRequested reports whether a cancellation is pending for this job.
func (s *JobService) cancelRequested(id uint) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cancelling[id]
}

// failReason is an error that carries a reason code and nothing else.
type failReason struct{ code string }

func (f failReason) Error() string { return "job failed: " + f.code }

// FailReason returns an error a worker can fail with to name the reason stored
// against the job. The code is validated when it is written; anything that is
// not a code becomes the generic worker error.
func FailReason(code string) error { return failReason{code: code} }

// reasonOf reduces a worker error to a storable code. Error text never reaches
// the job record: redaction by construction, because the string that leaks
// candidate content is always the one nobody thought about.
func reasonOf(err error) string {
	var fr failReason
	if errors.As(err, &fr) && models.ValidReason(fr.code) {
		return fr.code
	}
	return models.ReasonWorkerError
}

// safeCall turns a panicking worker into a failed job. The recruiter loses one
// job, not the application.
func safeCall(fn func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = failReason{code: models.ReasonPanic}
		}
	}()
	return fn()
}

// safeWork is safeCall for the compute half, which also returns a closure.
func safeWork(fn func() (JobCommit, error)) (commit JobCommit, err error) {
	defer func() {
		if r := recover(); r != nil {
			commit, err = nil, failReason{code: models.ReasonPanic}
		}
	}()
	return fn()
}

// demoParams is what the demo worker accepts: how long an item takes, and which
// item to fail on. It exists so the UI and the E2E tests have a job to drive.
type demoParams struct {
	DelayMS int `json:"delayMs"`
	FailAt  int `json:"failAt"`
}

// demoWorker is the only worker this phase registers: it sleeps, and fails on
// demand. Real pipelines arrive with the phases that need them.
func demoWorker(ctx context.Context, job models.Job, item int) (JobCommit, error) {
	p := demoParams{DelayMS: 100, FailAt: -1}
	if err := json.Unmarshal([]byte(job.Params), &p); err != nil {
		return nil, FailReason("bad_params")
	}
	if p.FailAt == item {
		return nil, FailReason("demo_failure")
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(time.Duration(p.DelayMS) * time.Millisecond):
		return nil, nil
	}
}
