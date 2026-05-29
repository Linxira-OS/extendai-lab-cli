package model

import (
	"testing"
	"time"
)

func TestNewJobManager(t *testing.T) {
	jm := NewJobManager()
	if jm == nil {
		t.Fatal("NewJobManager() returned nil")
	}
	if len(jm.ListJobs()) != 0 {
		t.Errorf("NewJobManager() should have 0 jobs, got %d", len(jm.ListJobs()))
	}
}

func TestJobManagerStartJob(t *testing.T) {
	jm := NewJobManager()

	jobID, err := jm.StartJob("echo hello", "test job")
	if err != nil {
		t.Fatalf("StartJob() error = %v", err)
	}
	if jobID == "" {
		t.Fatal("StartJob() returned empty job ID")
	}

	// Job should be running
	job := jm.GetJob(jobID)
	if job == nil {
		t.Fatal("GetJob() returned nil")
	}
	if job.Status != JobRunning {
		t.Errorf("job.Status = %v, want %v", job.Status, JobRunning)
	}
	if job.Command != "echo hello" {
		t.Errorf("job.Command = %q, want %q", job.Command, "echo hello")
	}
	if job.Description != "test job" {
		t.Errorf("job.Description = %q, want %q", job.Description, "test job")
	}
}

func TestJobManagerWaitJob(t *testing.T) {
	jm := NewJobManager()

	jobID, _ := jm.StartJob("echo hello", "test")
	job := jm.WaitJob(jobID)

	if job == nil {
		t.Fatal("WaitJob() returned nil")
	}
	if job.Status != JobCompleted {
		t.Errorf("job.Status = %v, want %v", job.Status, JobCompleted)
	}
	if job.Output != "hello\n" {
		t.Errorf("job.Output = %q, want %q", job.Output, "hello\n")
	}
	if job.ExitCode != 0 {
		t.Errorf("job.ExitCode = %d, want 0", job.ExitCode)
	}
}

func TestJobManagerWaitJobFailed(t *testing.T) {
	jm := NewJobManager()

	jobID, _ := jm.StartJob("exit 1", "failing job")
	job := jm.WaitJob(jobID)

	if job == nil {
		t.Fatal("WaitJob() returned nil")
	}
	if job.Status != JobFailed {
		t.Errorf("job.Status = %v, want %v", job.Status, JobFailed)
	}
	if job.ExitCode != 1 {
		t.Errorf("job.ExitCode = %d, want 1", job.ExitCode)
	}
}

func TestJobManagerCancelJob(t *testing.T) {
	jm := NewJobManager()

	// Start a long-running job
	jobID, _ := jm.StartJob("sleep 10", "long job")

	// Give it a moment to start
	time.Sleep(10 * time.Millisecond)

	// Cancel it
	ok := jm.CancelJob(jobID)
	if !ok {
		t.Fatal("CancelJob() returned false")
	}

	job := jm.GetJob(jobID)
	if job.Status != JobCancelled {
		t.Errorf("job.Status = %v, want %v", job.Status, JobCancelled)
	}
}

func TestJobManagerCancelNonexistentJob(t *testing.T) {
	jm := NewJobManager()

	ok := jm.CancelJob("nonexistent")
	if ok {
		t.Error("CancelJob() should return false for nonexistent job")
	}
}

func TestJobManagerCancelCompletedJob(t *testing.T) {
	jm := NewJobManager()

	jobID, _ := jm.StartJob("echo done", "quick job")
	jm.WaitJob(jobID)

	ok := jm.CancelJob(jobID)
	if ok {
		t.Error("CancelJob() should return false for completed job")
	}
}

func TestJobManagerListJobs(t *testing.T) {
	jm := NewJobManager()

	jm.StartJob("echo 1", "job 1")
	jm.StartJob("echo 2", "job 2")
	jm.StartJob("echo 3", "job 3")

	jobs := jm.ListJobs()
	if len(jobs) != 3 {
		t.Errorf("ListJobs() returned %d jobs, want 3", len(jobs))
	}
}

func TestJobManagerGetJob(t *testing.T) {
	jm := NewJobManager()

	jobID, _ := jm.StartJob("echo test", "test")

	job := jm.GetJob(jobID)
	if job == nil {
		t.Fatal("GetJob() returned nil")
	}

	nonexistent := jm.GetJob("nonexistent")
	if nonexistent != nil {
		t.Error("GetJob() should return nil for nonexistent job")
	}
}

func TestJobFormatJobStatus(t *testing.T) {
	jm := NewJobManager()

	jobID, _ := jm.StartJob("echo hello", "test job")
	job := jm.WaitJob(jobID)

	status := job.FormatJobStatus()
	if status == "" {
		t.Fatal("FormatJobStatus() returned empty string")
	}

	// Check that status contains expected fields
	expected := []string{jobID, "completed", "echo hello", "test job"}
	for _, exp := range expected {
		if !containsStr(status, exp) {
			t.Errorf("FormatJobStatus() should contain %q", exp)
		}
	}
}

func TestJobManagerFormatJobList(t *testing.T) {
	jm := NewJobManager()

	// Empty list
	list := jm.FormatJobList()
	if list != "No background jobs." {
		t.Errorf("FormatJobList() = %q, want %q", list, "No background jobs.")
	}

	// With jobs
	jm.StartJob("echo 1", "job 1")
	jm.StartJob("echo 2", "job 2")

	list = jm.FormatJobList()
	if list == "" {
		t.Fatal("FormatJobList() returned empty string")
	}
	if !containsStr(list, "Background jobs:") {
		t.Error("FormatJobList() should contain 'Background jobs:'")
	}
}

func TestJobStatusString(t *testing.T) {
	tests := []struct {
		status JobStatus
		want   string
	}{
		{JobRunning, "running"},
		{JobCompleted, "completed"},
		{JobFailed, "failed"},
		{JobCancelled, "cancelled"},
		{JobStatus(99), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.status.String(); got != tt.want {
			t.Errorf("JobStatus(%d).String() = %q, want %q", tt.status, got, tt.want)
		}
	}
}

func TestJobManagerConcurrency(t *testing.T) {
	jm := NewJobManager()

	// Start multiple jobs concurrently
	done := make(chan string, 5)
	for i := 0; i < 5; i++ {
		go func(n int) {
			jobID, _ := jm.StartJob("echo test", "concurrent job")
			done <- jobID
		}(i)
	}

	// Collect job IDs
	var jobIDs []string
	for i := 0; i < 5; i++ {
		jobIDs = append(jobIDs, <-done)
	}

	// Wait for all jobs
	for _, id := range jobIDs {
		jm.WaitJob(id)
	}

	// Verify all jobs completed
	jobs := jm.ListJobs()
	if len(jobs) != 5 {
		t.Errorf("ListJobs() returned %d jobs, want 5", len(jobs))
	}
	for _, job := range jobs {
		if job.Status != JobCompleted {
			t.Errorf("job %s status = %v, want %v", job.ID, job.Status, JobCompleted)
		}
	}
}
