package clustersync

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/event"
	"github.com/google/uuid"
	"github.com/uozi-tech/cosy/logger"
)

// EventTypeTask carries the progress and the outcome of a background sync task
// to the UI.
const EventTypeTask = event.Type("cluster_sync_task")

// TaskStatus is the lifecycle state of a background sync task.
type TaskStatus string

const (
	TaskRunning  TaskStatus = "running"
	TaskFinished TaskStatus = "finished"
)

// taskRetention keeps a finished task readable long enough for a browser that
// missed the completion event to fetch the outcome.
const taskRetention = time.Hour

// progressInterval throttles progress events. A large sync reports thousands
// of results and every event is broadcast to every connected browser.
const progressInterval = 500 * time.Millisecond

// Task is a sync run executed in the background. A sync pushes every site to
// every node and each push tests and reloads Nginx there, so a namespace with a
// few dozen sites easily outlives the timeout of the reverse proxy in front of
// Nginx UI.
type Task struct {
	ID     string     `json:"id"`
	Label  string     `json:"label"`
	Status TaskStatus `json:"status"`
	// Done and Total count the item-on-node pushes of the run.
	Done       int        `json:"done"`
	Total      int        `json:"total"`
	Summary    *Summary   `json:"summary,omitempty"`
	StartedAt  time.Time  `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`

	// key identifies the work, so a second click on the same namespace joins
	// the running task instead of pushing everything twice.
	key string
}

type taskRegistry struct {
	mutex sync.Mutex
	tasks map[string]*Task
}

var tasks = &taskRegistry{tasks: map[string]*Task{}}

// GetTask returns a snapshot of a task that is running or finished recently.
func GetTask(id string) (*Task, error) {
	tasks.mutex.Lock()
	defer tasks.mutex.Unlock()

	task, ok := tasks.tasks[id]
	if !ok {
		return nil, ErrTaskNotFound
	}

	return task.snapshot(), nil
}

// StartNamespaceSync validates the namespace sync and runs it in the
// background. Validation errors are returned right away.
func StartNamespaceSync(namespaceID uint64) (*Task, error) {
	namespace, nodes, items, err := prepareNamespace(namespaceID)
	if err != nil {
		return nil, err
	}

	return startTask(fmt.Sprintf("namespace:%d", namespace.ID), namespace.Name, nodes, items), nil
}

// StartNodesSync validates the node sync and runs it in the background.
// Validation errors are returned right away.
func StartNodesSync(nodeIDs []uint64, scope Scope) (*Task, error) {
	nodes, items, err := prepareNodes(nodeIDs, scope)
	if err != nil {
		return nil, err
	}

	ids := make([]uint64, 0, len(nodes))
	for _, node := range nodes {
		ids = append(ids, node.id)
	}

	return startTask(fmt.Sprintf("nodes:%v:%+v", ids, scope), "nodes", nodes, items), nil
}

func startTask(key, label string, nodes []nodeRef, items []item) *Task {
	tasks.mutex.Lock()
	defer tasks.mutex.Unlock()

	tasks.pruneLocked(time.Now())
	for _, task := range tasks.tasks {
		if task.key == key && task.Status == TaskRunning {
			return task.snapshot()
		}
	}

	task := &Task{
		ID:        uuid.NewString(),
		Label:     label,
		Status:    TaskRunning,
		Total:     len(nodes) * len(items),
		StartedAt: time.Now(),
		key:       key,
	}
	tasks.tasks[task.ID] = task

	go tasks.execute(task, nodes, items)

	return task.snapshot()
}

// execute runs detached from the request that started it: the browser or the
// proxy giving up on the request must not abort a half-applied sync.
func (r *taskRegistry) execute(task *Task, nodes []nodeRef, items []item) {
	summary := &Summary{Results: []Result{}, Skipped: []SkippedFile{}}
	defer func() {
		if err := recover(); err != nil {
			buf := make([]byte, 1024)
			runtime.Stack(buf, false)
			logger.Errorf("cluster sync task %s panicked: %v\n%s", task.ID, err, buf)
		}
		r.finish(task, summary)
	}()

	// Nodes report concurrently, so the throttle state lives under the lock.
	var lastPublish time.Time
	summary = runWithProgress(context.Background(), nodes, items, func(done int) {
		r.mutex.Lock()
		task.Done = max(task.Done, done)
		publish := time.Since(lastPublish) >= progressInterval
		if publish {
			lastPublish = time.Now()
		}
		snapshot := task.snapshot()
		r.mutex.Unlock()

		if publish {
			publishTask(snapshot)
		}
	})
}

func (r *taskRegistry) finish(task *Task, summary *Summary) {
	r.mutex.Lock()
	now := time.Now()
	task.Status = TaskFinished
	task.Summary = summary
	task.Done = task.Total
	task.FinishedAt = &now
	snapshot := task.snapshot()
	r.mutex.Unlock()

	publishTask(snapshot)
}

// pruneLocked drops finished tasks past their retention. Callers hold the lock.
func (r *taskRegistry) pruneLocked(now time.Time) {
	for id, task := range r.tasks {
		if task.FinishedAt != nil && now.Sub(*task.FinishedAt) > taskRetention {
			delete(r.tasks, id)
		}
	}
}

// snapshot copies the task so it can leave the lock. The summary is written
// once when the task finishes and never changed afterwards.
func (t *Task) snapshot() *Task {
	copied := *t
	return &copied
}

func publishTask(task *Task) {
	event.Publish(event.Event{Type: EventTypeTask, Data: task})
}
