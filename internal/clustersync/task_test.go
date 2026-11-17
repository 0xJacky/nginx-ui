package clustersync

import (
	"context"
	"errors"
	"testing"
	"time"
)

func waitForTask(t *testing.T, id string) *Task {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		task, err := GetTask(id)
		if err != nil {
			t.Fatalf("get task: %v", err)
		}
		if task.Status == TaskFinished {
			return task
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("task %s did not finish in time", id)
	return nil
}

func TestStartTaskReturnsBeforeTheSyncFinishes(t *testing.T) {
	release := make(chan struct{})
	items := []item{
		{kind: KindSite, name: "slow.example", push: func(context.Context, nodeRef) error {
			<-release
			return nil
		}},
		{kind: KindSite, name: "broken.example", push: func(context.Context, nodeRef) error {
			return errors.New("nginx test failed")
		}},
	}

	task := startTask(t.Name(), "production", []nodeRef{{id: 1, name: "n1"}}, items)
	if task.Status != TaskRunning || task.Total != 2 || task.Summary != nil {
		t.Fatalf("expected a running task with two pushes, got %+v", task)
	}

	close(release)
	finished := waitForTask(t, task.ID)
	if finished.Done != 2 || finished.FinishedAt == nil {
		t.Fatalf("unexpected finished task %+v", finished)
	}
	if finished.Summary == nil || finished.Summary.Succeeded != 1 || finished.Summary.Failed != 1 {
		t.Fatalf("unexpected summary %+v", finished.Summary)
	}
}

func TestStartTaskJoinsTheRunningTaskForTheSameWork(t *testing.T) {
	release := make(chan struct{})
	calls := 0
	items := []item{{kind: KindSite, name: "a.example", push: func(context.Context, nodeRef) error {
		calls++
		<-release
		return nil
	}}}
	nodes := []nodeRef{{id: 1, name: "n1"}}

	first := startTask(t.Name(), "production", nodes, items)
	second := startTask(t.Name(), "production", nodes, items)
	if first.ID != second.ID {
		t.Fatalf("a second click must join the running task, got %s and %s", first.ID, second.ID)
	}

	close(release)
	waitForTask(t, first.ID)
	if calls != 1 {
		t.Fatalf("the work ran %d times", calls)
	}

	// Once finished, the same work starts a fresh run.
	third := startTask(t.Name(), "production", nodes, items)
	if third.ID == first.ID {
		t.Fatal("a finished task must not be reused")
	}
	waitForTask(t, third.ID)
}

func TestGetTaskReportsUnknownTasks(t *testing.T) {
	if _, err := GetTask("missing"); !errors.Is(err, ErrTaskNotFound) {
		t.Fatalf("expected ErrTaskNotFound, got %v", err)
	}
}

func TestPruneDropsExpiredFinishedTasksOnly(t *testing.T) {
	registry := &taskRegistry{tasks: map[string]*Task{}}
	now := time.Now()
	expired := now.Add(-taskRetention - time.Minute)
	recent := now.Add(-time.Minute)
	registry.tasks["expired"] = &Task{ID: "expired", Status: TaskFinished, FinishedAt: &expired}
	registry.tasks["recent"] = &Task{ID: "recent", Status: TaskFinished, FinishedAt: &recent}
	registry.tasks["running"] = &Task{ID: "running", Status: TaskRunning}

	registry.pruneLocked(now)

	if _, ok := registry.tasks["expired"]; ok {
		t.Fatal("an expired task must be dropped")
	}
	if len(registry.tasks) != 2 {
		t.Fatalf("unexpected remaining tasks %+v", registry.tasks)
	}
}
