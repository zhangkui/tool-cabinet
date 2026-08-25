package cabinet

import (
	"context"
	"errors"
	"sync"
	"time"
)

type JobStatus string

const (
	JobQueued    JobStatus = "queued"
	JobRunning   JobStatus = "running"
	JobSucceeded JobStatus = "succeeded"
	JobFailed    JobStatus = "failed"
	JobSkipped   JobStatus = "skipped"
)

type Job struct {
	ID          string
	Name        string
	Status      JobStatus
	ScheduledAt time.Time
	StartedAt   *time.Time
	FinishedAt  *time.Time
	Attempts    int
	LastError   string
}
type Scheduler struct {
	mu       sync.Mutex
	jobs     map[string]Job
	sequence uint64
	running  bool
}

var ErrJobExists = errors.New("job already exists")
var ErrSchedulerBusy = errors.New("scheduler is busy")

func NewScheduler() *Scheduler { return &Scheduler{jobs: make(map[string]Job)} }
func (s *Scheduler) Enqueue(name string, at time.Time) (Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, job := range s.jobs {
		if job.Name == name && job.Status == JobQueued {
			return Job{}, ErrJobExists
		}
	}
	s.sequence++
	job := Job{ID: "job-" + formatSequence(s.sequence), Name: name, Status: JobQueued, ScheduledAt: at}
	s.jobs[job.ID] = job
	return job, nil
}
func (s *Scheduler) Claim(now time.Time) (Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		return Job{}, ErrSchedulerBusy
	}
	var selected Job
	found := false
	for _, job := range s.jobs {
		if job.Status != JobQueued || job.ScheduledAt.After(now) {
			continue
		}
		if !found || job.ScheduledAt.Before(selected.ScheduledAt) {
			selected = job
			found = true
		}
	}
	if !found {
		return Job{}, nil
	}
	selected.Status = JobRunning
	selected.Attempts++
	selected.StartedAt = timePtr(now)
	s.jobs[selected.ID] = selected
	s.running = true
	return selected, nil
}
func (s *Scheduler) Complete(id string, err error, now time.Time) (Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[id]
	if !ok {
		return Job{}, errors.New("job not found")
	}
	if job.Status != JobRunning {
		return Job{}, errors.New("job is not running")
	}
	job.FinishedAt = timePtr(now)
	if err != nil {
		job.Status = JobFailed
		job.LastError = err.Error()
	} else {
		job.Status = JobSucceeded
	}
	s.jobs[id] = job
	s.running = false
	return job, nil
}
func (s *Scheduler) Skip(id string, reason string, now time.Time) (Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[id]
	if !ok {
		return Job{}, errors.New("job not found")
	}
	if job.Status != JobQueued {
		return Job{}, errors.New("job cannot be skipped")
	}
	job.Status = JobSkipped
	job.LastError = reason
	job.FinishedAt = timePtr(now)
	s.jobs[id] = job
	return job, nil
}
func (s *Scheduler) Get(id string) (Job, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[id]
	return job, ok
}
func (s *Scheduler) List(status JobStatus) []Job {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]Job, 0)
	for _, job := range s.jobs {
		if status == "" || job.Status == status {
			result = append(result, job)
		}
	}
	return result
}
func (s *Scheduler) RunOnce(ctx context.Context, now time.Time, handler func(context.Context, Job) error) (Job, error) {
	job, err := s.Claim(now)
	if err != nil || job.ID == "" {
		return job, err
	}
	runErr := handler(ctx, job)
	return s.Complete(job.ID, runErr, time.Now())
}
