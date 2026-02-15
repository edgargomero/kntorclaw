package cron

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func tempStorePath(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	return filepath.Join(dir, "cron", "jobs.json")
}

func TestNewCronService_EmptyStore(t *testing.T) {
	cs := NewCronService(tempStorePath(t), nil)
	jobs := cs.ListJobs(true)
	if len(jobs) != 0 {
		t.Errorf("new service should have 0 jobs, got %d", len(jobs))
	}
}

func TestAddJob_EverySchedule(t *testing.T) {
	cs := NewCronService(tempStorePath(t), nil)

	everyMS := int64(60000) // 60s
	job, err := cs.AddJob("check-health", CronSchedule{
		Kind:    "every",
		EveryMS: &everyMS,
	}, "check server health", false, "", "")

	if err != nil {
		t.Fatalf("AddJob() error: %v", err)
	}
	if job.Name != "check-health" {
		t.Errorf("Name = %q, want %q", job.Name, "check-health")
	}
	if job.ID == "" {
		t.Error("job ID should not be empty")
	}
	if !job.Enabled {
		t.Error("new job should be enabled by default")
	}
	if job.Payload.Message != "check server health" {
		t.Errorf("Payload.Message = %q, want %q", job.Payload.Message, "check server health")
	}
	if job.State.NextRunAtMS == nil {
		t.Error("NextRunAtMS should be set for 'every' schedule")
	}
	if job.DeleteAfterRun {
		t.Error("'every' schedule should not be delete-after-run")
	}
}

func TestAddJob_CronSchedule(t *testing.T) {
	cs := NewCronService(tempStorePath(t), nil)

	job, err := cs.AddJob("daily-report", CronSchedule{
		Kind: "cron",
		Expr: "0 9 * * *",
	}, "generate daily report", true, "telegram", "user123")

	if err != nil {
		t.Fatalf("AddJob() error: %v", err)
	}
	if job.Schedule.Expr != "0 9 * * *" {
		t.Errorf("Schedule.Expr = %q, want %q", job.Schedule.Expr, "0 9 * * *")
	}
	if !job.Payload.Deliver {
		t.Error("Deliver should be true")
	}
	if job.Payload.Channel != "telegram" {
		t.Errorf("Channel = %q, want %q", job.Payload.Channel, "telegram")
	}
	if job.State.NextRunAtMS == nil {
		t.Error("NextRunAtMS should be set for cron schedule")
	}
}

func TestAddJob_AtSchedule_DeleteAfterRun(t *testing.T) {
	cs := NewCronService(tempStorePath(t), nil)

	futureMS := time.Now().Add(1 * time.Hour).UnixMilli()
	job, err := cs.AddJob("one-time", CronSchedule{
		Kind: "at",
		AtMS: &futureMS,
	}, "remind me", false, "", "")

	if err != nil {
		t.Fatalf("AddJob() error: %v", err)
	}
	if !job.DeleteAfterRun {
		t.Error("'at' schedule should have DeleteAfterRun=true")
	}
}

func TestAddJob_PersistsToFile(t *testing.T) {
	storePath := tempStorePath(t)
	cs := NewCronService(storePath, nil)

	everyMS := int64(30000)
	_, err := cs.AddJob("persist-test", CronSchedule{
		Kind:    "every",
		EveryMS: &everyMS,
	}, "test persistence", false, "", "")
	if err != nil {
		t.Fatalf("AddJob() error: %v", err)
	}

	// Read the file directly
	data, err := os.ReadFile(storePath)
	if err != nil {
		t.Fatalf("failed to read store file: %v", err)
	}

	var store CronStore
	if err := json.Unmarshal(data, &store); err != nil {
		t.Fatalf("failed to parse store file: %v", err)
	}

	if len(store.Jobs) != 1 {
		t.Fatalf("store should have 1 job, got %d", len(store.Jobs))
	}
	if store.Jobs[0].Name != "persist-test" {
		t.Errorf("persisted job name = %q, want %q", store.Jobs[0].Name, "persist-test")
	}
}

func TestRemoveJob(t *testing.T) {
	cs := NewCronService(tempStorePath(t), nil)

	everyMS := int64(10000)
	job, _ := cs.AddJob("removable", CronSchedule{Kind: "every", EveryMS: &everyMS}, "msg", false, "", "")

	if !cs.RemoveJob(job.ID) {
		t.Error("RemoveJob should return true for existing job")
	}

	if cs.RemoveJob(job.ID) {
		t.Error("RemoveJob should return false when job already removed")
	}

	jobs := cs.ListJobs(true)
	if len(jobs) != 0 {
		t.Errorf("should have 0 jobs after removal, got %d", len(jobs))
	}
}

func TestRemoveJob_NonExistent(t *testing.T) {
	cs := NewCronService(tempStorePath(t), nil)
	if cs.RemoveJob("nonexistent-id") {
		t.Error("RemoveJob should return false for non-existent job")
	}
}

func TestEnableJob(t *testing.T) {
	cs := NewCronService(tempStorePath(t), nil)

	everyMS := int64(10000)
	job, _ := cs.AddJob("toggleable", CronSchedule{Kind: "every", EveryMS: &everyMS}, "msg", false, "", "")

	// Disable
	result := cs.EnableJob(job.ID, false)
	if result == nil {
		t.Fatal("EnableJob returned nil")
	}
	if result.Enabled {
		t.Error("job should be disabled")
	}
	if result.State.NextRunAtMS != nil {
		t.Error("disabled job should have nil NextRunAtMS")
	}

	// Re-enable
	result = cs.EnableJob(job.ID, true)
	if result == nil {
		t.Fatal("EnableJob returned nil")
	}
	if !result.Enabled {
		t.Error("job should be enabled")
	}
	if result.State.NextRunAtMS == nil {
		t.Error("re-enabled job should have NextRunAtMS set")
	}
}

func TestEnableJob_NonExistent(t *testing.T) {
	cs := NewCronService(tempStorePath(t), nil)
	result := cs.EnableJob("nonexistent", true)
	if result != nil {
		t.Error("EnableJob should return nil for non-existent job")
	}
}

func TestListJobs_FilterDisabled(t *testing.T) {
	cs := NewCronService(tempStorePath(t), nil)

	everyMS := int64(10000)
	cs.AddJob("job1", CronSchedule{Kind: "every", EveryMS: &everyMS}, "msg1", false, "", "")
	job2, _ := cs.AddJob("job2", CronSchedule{Kind: "every", EveryMS: &everyMS}, "msg2", false, "", "")
	cs.AddJob("job3", CronSchedule{Kind: "every", EveryMS: &everyMS}, "msg3", false, "", "")

	cs.EnableJob(job2.ID, false)

	allJobs := cs.ListJobs(true)
	if len(allJobs) != 3 {
		t.Errorf("ListJobs(true) = %d jobs, want 3", len(allJobs))
	}

	enabledJobs := cs.ListJobs(false)
	if len(enabledJobs) != 2 {
		t.Errorf("ListJobs(false) = %d jobs, want 2", len(enabledJobs))
	}
}

func TestUpdateJob(t *testing.T) {
	cs := NewCronService(tempStorePath(t), nil)

	everyMS := int64(10000)
	job, _ := cs.AddJob("updatable", CronSchedule{Kind: "every", EveryMS: &everyMS}, "original", false, "", "")

	job.Payload.Message = "updated message"
	err := cs.UpdateJob(job)
	if err != nil {
		t.Fatalf("UpdateJob() error: %v", err)
	}

	jobs := cs.ListJobs(true)
	if jobs[0].Payload.Message != "updated message" {
		t.Errorf("updated message = %q, want %q", jobs[0].Payload.Message, "updated message")
	}
}

func TestUpdateJob_NonExistent(t *testing.T) {
	cs := NewCronService(tempStorePath(t), nil)
	err := cs.UpdateJob(&CronJob{ID: "ghost"})
	if err == nil {
		t.Error("UpdateJob should return error for non-existent job")
	}
}

func TestComputeNextRun_Every(t *testing.T) {
	cs := NewCronService(tempStorePath(t), nil)
	now := time.Now().UnixMilli()

	everyMS := int64(5000)
	schedule := &CronSchedule{Kind: "every", EveryMS: &everyMS}

	next := cs.computeNextRun(schedule, now)
	if next == nil {
		t.Fatal("computeNextRun returned nil for 'every'")
	}

	expected := now + 5000
	if *next != expected {
		t.Errorf("next = %d, want %d", *next, expected)
	}
}

func TestComputeNextRun_Every_ZeroInterval(t *testing.T) {
	cs := NewCronService(tempStorePath(t), nil)

	zeroMS := int64(0)
	schedule := &CronSchedule{Kind: "every", EveryMS: &zeroMS}

	next := cs.computeNextRun(schedule, time.Now().UnixMilli())
	if next != nil {
		t.Error("computeNextRun should return nil for zero interval")
	}
}

func TestComputeNextRun_At_Future(t *testing.T) {
	cs := NewCronService(tempStorePath(t), nil)

	now := time.Now().UnixMilli()
	futureMS := now + 60000
	schedule := &CronSchedule{Kind: "at", AtMS: &futureMS}

	next := cs.computeNextRun(schedule, now)
	if next == nil {
		t.Fatal("computeNextRun returned nil for future 'at'")
	}
	if *next != futureMS {
		t.Errorf("next = %d, want %d", *next, futureMS)
	}
}

func TestComputeNextRun_At_Past(t *testing.T) {
	cs := NewCronService(tempStorePath(t), nil)

	now := time.Now().UnixMilli()
	pastMS := now - 60000
	schedule := &CronSchedule{Kind: "at", AtMS: &pastMS}

	next := cs.computeNextRun(schedule, now)
	if next != nil {
		t.Error("computeNextRun should return nil for past 'at' time")
	}
}

func TestComputeNextRun_Cron(t *testing.T) {
	cs := NewCronService(tempStorePath(t), nil)

	now := time.Now().UnixMilli()
	schedule := &CronSchedule{Kind: "cron", Expr: "* * * * *"} // every minute

	next := cs.computeNextRun(schedule, now)
	if next == nil {
		t.Fatal("computeNextRun returned nil for cron expression")
	}
	if *next <= now {
		t.Error("next run should be in the future")
	}
	// Should be within the next ~61 seconds
	maxNext := now + 61000
	if *next > maxNext {
		t.Errorf("next run %d is too far in the future (max %d)", *next, maxNext)
	}
}

func TestComputeNextRun_Cron_EmptyExpr(t *testing.T) {
	cs := NewCronService(tempStorePath(t), nil)

	schedule := &CronSchedule{Kind: "cron", Expr: ""}
	next := cs.computeNextRun(schedule, time.Now().UnixMilli())
	if next != nil {
		t.Error("computeNextRun should return nil for empty cron expression")
	}
}

func TestComputeNextRun_UnknownKind(t *testing.T) {
	cs := NewCronService(tempStorePath(t), nil)

	schedule := &CronSchedule{Kind: "unknown"}
	next := cs.computeNextRun(schedule, time.Now().UnixMilli())
	if next != nil {
		t.Error("computeNextRun should return nil for unknown kind")
	}
}

func TestStatus(t *testing.T) {
	cs := NewCronService(tempStorePath(t), nil)

	everyMS := int64(10000)
	cs.AddJob("status-test", CronSchedule{Kind: "every", EveryMS: &everyMS}, "msg", false, "", "")

	status := cs.Status()
	if status["jobs"].(int) != 1 {
		t.Errorf("Status jobs = %v, want 1", status["jobs"])
	}
}

func TestStartAndStop(t *testing.T) {
	cs := NewCronService(tempStorePath(t), nil)

	if err := cs.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	// Double start should be idempotent
	if err := cs.Start(); err != nil {
		t.Fatalf("second Start() error: %v", err)
	}

	cs.Stop()

	// Double stop should be safe
	cs.Stop()
}

func TestLoadFromExistingStore(t *testing.T) {
	storePath := tempStorePath(t)
	cs1 := NewCronService(storePath, nil)

	everyMS := int64(10000)
	cs1.AddJob("persistent", CronSchedule{Kind: "every", EveryMS: &everyMS}, "msg", false, "", "")

	// Create a new service pointing to the same file
	cs2 := NewCronService(storePath, nil)
	jobs := cs2.ListJobs(true)
	if len(jobs) != 1 {
		t.Fatalf("reloaded service should have 1 job, got %d", len(jobs))
	}
	if jobs[0].Name != "persistent" {
		t.Errorf("reloaded job name = %q, want %q", jobs[0].Name, "persistent")
	}
}

func TestGenerateID_Unique(t *testing.T) {
	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := generateID()
		if id == "" {
			t.Fatal("generateID returned empty string")
		}
		if ids[id] {
			t.Fatalf("generateID returned duplicate: %s", id)
		}
		ids[id] = true
	}
}

func TestSetOnJob(t *testing.T) {
	cs := NewCronService(tempStorePath(t), nil)

	called := false
	cs.SetOnJob(func(job *CronJob) (string, error) {
		called = true
		return "done", nil
	})

	// Verify handler is set (indirectly through executeJobByID)
	everyMS := int64(10000)
	job, _ := cs.AddJob("handler-test", CronSchedule{Kind: "every", EveryMS: &everyMS}, "msg", false, "", "")
	cs.executeJobByID(job.ID)

	if !called {
		t.Error("onJob handler was not called")
	}
}

func TestConcurrentAddAndList(t *testing.T) {
	cs := NewCronService(tempStorePath(t), nil)
	everyMS := int64(10000)

	var wg sync.WaitGroup
	const n = 20

	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			cs.AddJob("concurrent", CronSchedule{Kind: "every", EveryMS: &everyMS}, "msg", false, "", "")
		}(i)
	}

	// Also read concurrently
	wg.Add(5)
	for i := 0; i < 5; i++ {
		go func() {
			defer wg.Done()
			cs.ListJobs(true)
		}()
	}

	wg.Wait()

	jobs := cs.ListJobs(true)
	if len(jobs) != n {
		t.Errorf("expected %d jobs after concurrent adds, got %d", n, len(jobs))
	}
}
