package model

import (
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// ─── Job Manager (background process management) ───────────
// Inspired by oh-my-pi's AsyncJobManager and opencode-pty's PTYManager

type JobStatus int

const (
	JobRunning JobStatus = iota
	JobCompleted
	JobFailed
	JobCancelled
)

func (s JobStatus) String() string {
	switch s {
	case JobRunning:
		return "running"
	case JobCompleted:
		return "completed"
	case JobFailed:
		return "failed"
	case JobCancelled:
		return "cancelled"
	}
	return "unknown"
}

type Job struct {
	ID          string
	Command     string
	Description string
	Status      JobStatus
	StartedAt   time.Time
	FinishedAt  time.Time
	ExitCode    int
	Output      string // accumulated output
	Error       string
	cmd         *exec.Cmd
	done        chan struct{}
}

type JobManager struct {
	mu   sync.Mutex
	jobs map[string]*Job
	seq  int
}

func NewJobManager() *JobManager {
	return &JobManager{
		jobs: make(map[string]*Job),
	}
}

// StartJob starts a background command and returns the job ID.
func (jm *JobManager) StartJob(command string, description string) (string, error) {
	jm.mu.Lock()
	jm.seq++
	jobID := fmt.Sprintf("job_%d", jm.seq)
	jm.mu.Unlock()

	// Use shell to execute the command
	shell := "/bin/sh"
	if _, err := exec.LookPath("bash"); err == nil {
		shell = "bash"
	}

	cmd := exec.Command(shell, "-c", command)
	cmd.Dir = "" // will be set by caller

	job := &Job{
		ID:          jobID,
		Command:     command,
		Description: description,
		Status:      JobRunning,
		StartedAt:   time.Now(),
		cmd:         cmd,
		done:        make(chan struct{}),
	}

	jm.mu.Lock()
	jm.jobs[jobID] = job
	jm.mu.Unlock()

	// Start the command in background
	go func() {
		defer close(job.done)

		output, err := cmd.CombinedOutput()
		job.Output = string(output)

		jm.mu.Lock()
		job.FinishedAt = time.Now()
		if err != nil {
			job.Status = JobFailed
			job.Error = err.Error()
			if exitErr, ok := err.(*exec.ExitError); ok {
				job.ExitCode = exitErr.ExitCode()
			}
		} else {
			job.Status = JobCompleted
		}
		jm.mu.Unlock()
	}()

	return jobID, nil
}

// GetJob returns a job by ID.
func (jm *JobManager) GetJob(id string) *Job {
	jm.mu.Lock()
	defer jm.mu.Unlock()
	return jm.jobs[id]
}

// ListJobs returns all jobs.
func (jm *JobManager) ListJobs() []*Job {
	jm.mu.Lock()
	defer jm.mu.Unlock()
	jobs := make([]*Job, 0, len(jm.jobs))
	for _, j := range jm.jobs {
		jobs = append(jobs, j)
	}
	return jobs
}

// CancelJob cancels a running job.
func (jm *JobManager) CancelJob(id string) bool {
	jm.mu.Lock()
	job, ok := jm.jobs[id]
	jm.mu.Unlock()
	if !ok || job.Status != JobRunning {
		return false
	}

	if job.cmd != nil && job.cmd.Process != nil {
		job.cmd.Process.Kill()
	}

	jm.mu.Lock()
	job.Status = JobCancelled
	job.FinishedAt = time.Now()
	jm.mu.Unlock()

	return true
}

// WaitJob blocks until a job completes.
func (jm *JobManager) WaitJob(id string) *Job {
	jm.mu.Lock()
	job, ok := jm.jobs[id]
	jm.mu.Unlock()
	if !ok {
		return nil
	}
	<-job.done
	return job
}

// FormatJobStatus returns a formatted status string for a job.
func (j *Job) FormatJobStatus() string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Job %s: %s\n", j.ID, j.Status))
	b.WriteString(fmt.Sprintf("Command: %s\n", j.Command))
	if j.Description != "" {
		b.WriteString(fmt.Sprintf("Description: %s\n", j.Description))
	}
	b.WriteString(fmt.Sprintf("Started: %s\n", j.StartedAt.Format("15:04:05")))
	if !j.FinishedAt.IsZero() {
		b.WriteString(fmt.Sprintf("Finished: %s\n", j.FinishedAt.Format("15:04:05")))
		b.WriteString(fmt.Sprintf("Duration: %s\n", j.FinishedAt.Sub(j.StartedAt).Round(time.Millisecond)))
	}
	if j.ExitCode != 0 {
		b.WriteString(fmt.Sprintf("Exit code: %d\n", j.ExitCode))
	}
	if j.Error != "" {
		b.WriteString(fmt.Sprintf("Error: %s\n", j.Error))
	}
	if j.Output != "" {
		// Truncate output for display
		output := j.Output
		if len(output) > 2000 {
			output = "...\n" + output[len(output)-2000:]
		}
		b.WriteString(fmt.Sprintf("Output:\n%s", output))
	}
	return b.String()
}

// FormatJobList returns a formatted list of all jobs.
func (jm *JobManager) FormatJobList() string {
	jobs := jm.ListJobs()
	if len(jobs) == 0 {
		return "No background jobs."
	}
	var b strings.Builder
	b.WriteString("Background jobs:\n")
	for _, j := range jobs {
		duration := ""
		if !j.FinishedAt.IsZero() {
			duration = j.FinishedAt.Sub(j.StartedAt).Round(time.Millisecond).String()
		} else {
			duration = time.Since(j.StartedAt).Round(time.Millisecond).String()
		}
		b.WriteString(fmt.Sprintf("  %s [%s] %s (%s)\n", j.ID, j.Status, j.Command, duration))
	}
	return b.String()
}
