package models

import (
	"fmt"
	"regexp"
	"slices"
	"time"
)

// JobState is where a background job has got to. A job is in exactly one of
// these, and three of them are final.
type JobState string

// The job states, in the order a job meets them.
const (
	JobQueued    JobState = "queued"
	JobRunning   JobState = "running"
	JobCompleted JobState = "completed"
	JobFailed    JobState = "failed"
	JobCancelled JobState = "cancelled"
)

// JobStates is every state, so tests can walk the whole transition matrix.
var JobStates = []JobState{JobQueued, JobRunning, JobCompleted, JobFailed, JobCancelled}

// allowed is the transition table, and the only place a transition is decided.
// Everything absent from it is refused — including a state to itself, because a
// job that is already running was not started twice.
var allowed = map[JobState][]JobState{
	JobQueued:  {JobRunning, JobCancelled},
	JobRunning: {JobCompleted, JobFailed, JobCancelled},
	// Completed, failed, and cancelled are final: a finished job has no next.
}

// Final reports whether s is a state a job never leaves on its own. Retry is
// not a transition; it puts the job back at the start of the table.
func (s JobState) Final() bool { return len(allowed[s]) == 0 }

// CanTransition reports whether a job may move from s to next.
func (s JobState) CanTransition(next JobState) bool {
	return slices.Contains(allowed[s], next)
}

// Valid reports whether s is a known state.
func (s JobState) Valid() bool { return slices.Contains(JobStates, s) }

// The failure reason codes this package defines. A worker may declare its own,
// as long as it is a code and not a sentence.
const (
	// ReasonInterrupted is a job the application found unfinished at start-up:
	// the process that was running it is gone.
	ReasonInterrupted = "interrupted"
	// ReasonWorkerError is any worker failure that did not declare a code.
	ReasonWorkerError = "worker_error"
	// ReasonPanic is a worker that panicked.
	ReasonPanic = "panic"
)

// reasonPattern is the whole redaction policy: a failure reason is a short
// lowercase code. A field that can hold a sentence can hold a candidate's name,
// so this one cannot hold a sentence.
var reasonPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,39}$`)

// ValidReason reports whether reason is a storable failure code.
func ValidReason(reason string) bool { return reasonPattern.MatchString(reason) }

// CheckReason returns an error naming the rule when reason is not a code.
func CheckReason(reason string) error {
	if !ValidReason(reason) {
		return fmt.Errorf("job failure reason must be a short lowercase code, got %q", reason)
	}
	return nil
}

// Job is one unit of slow work: its state, how far it got, and why it stopped.
// It carries no content — Params names records by ID, and FailureReason is a
// code — so a job record can be shown, logged, and exported freely.
type Job struct {
	ID   uint   `gorm:"primarykey" json:"id"`
	Kind string `gorm:"not null" json:"kind"`
	// The initiative whose workspace shows this job, if any. Nil for work that
	// belongs to no single initiative.
	InitiativeID *uint `json:"initiativeId"`
	// Worker input as JSON: identifiers and settings, never content.
	Params         string     `gorm:"not null;default:'{}'" json:"params"`
	State          JobState   `gorm:"not null" json:"state"`
	TotalItems     int        `gorm:"not null" json:"totalItems"`
	CompletedItems int        `gorm:"not null" json:"completedItems"`
	FailureReason  string     `json:"failureReason"`
	StartedAt      *time.Time `json:"startedAt"`
	FinishedAt     *time.Time `json:"finishedAt"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}
