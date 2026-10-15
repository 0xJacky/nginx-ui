package ingress

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/uozi-tech/cosy"
	"github.com/uozi-tech/cosy/logger"
	"gorm.io/gorm"
)

// Coalescing window of the worker: a batch starts once no change arrived for
// coalesceQuiet, and at the latest coalesceMax after the first change.
// Variables so tests can shorten them.
var (
	coalesceQuiet = time.Second
	coalesceMax   = 5 * time.Second
)

// deleteJob is a pending delete with everyone waiting for it.
type deleteJob struct {
	purgeCert bool
	done      []chan error
}

// workQueue holds the changes the worker has not picked up yet.
type workQueue struct {
	mu      sync.Mutex
	applies map[string]struct{}
	deletes map[string]*deleteJob
	signal  chan struct{}
}

var queue = &workQueue{
	applies: make(map[string]struct{}),
	deletes: make(map[string]*deleteJob),
	signal:  make(chan struct{}, 1),
}

func (q *workQueue) notify() {
	select {
	case q.signal <- struct{}{}:
	default:
	}
}

// take hands the pending changes to the worker and empties the queue.
func (q *workQueue) take() ([]string, map[string]*deleteJob) {
	q.mu.Lock()
	defer q.mu.Unlock()
	applies := make([]string, 0, len(q.applies))
	for name := range q.applies {
		applies = append(applies, name)
	}
	sort.Strings(applies)
	deletes := q.deletes
	q.applies = make(map[string]struct{})
	q.deletes = make(map[string]*deleteJob)
	return applies, deletes
}

// Enqueue asks the worker to apply the named ingresses. Calls that arrive
// close together are coalesced into one apply.
func Enqueue(names ...string) {
	if len(names) == 0 {
		return
	}
	queue.mu.Lock()
	for _, name := range names {
		queue.applies[name] = struct{}{}
	}
	queue.mu.Unlock()
	queue.notify()
}

// enqueueDelete asks the worker to remove an ingress and its files. The
// returned channel receives the outcome once.
func enqueueDelete(name string, purgeCert bool) <-chan error {
	done := make(chan error, 1)
	queue.mu.Lock()
	job, ok := queue.deletes[name]
	if !ok {
		job = &deleteJob{}
		queue.deletes[name] = job
	}
	job.purgeCert = job.purgeCert || purgeCert
	job.done = append(job.done, done)
	queue.mu.Unlock()
	queue.notify()
	return done
}

var (
	waitMu    sync.Mutex
	waitChans = make(map[string]chan struct{})
)

// waitChan returns the channel closed by the next notifyWaiters(name).
func waitChan(name string) <-chan struct{} {
	waitMu.Lock()
	defer waitMu.Unlock()
	ch, ok := waitChans[name]
	if !ok {
		ch = make(chan struct{})
		waitChans[name] = ch
	}
	return ch
}

// notifyWaiters wakes everyone waiting on the ingress.
func notifyWaiters(name string) {
	waitMu.Lock()
	defer waitMu.Unlock()
	if ch, ok := waitChans[name]; ok {
		close(ch)
		delete(waitChans, name)
	}
}

// WaitForGeneration blocks until the ingress has been applied at least at the
// given generation (successfully or not), or ctx ends.
func WaitForGeneration(ctx context.Context, name string, generation int64) (*model.Ingress, error) {
	for {
		// Subscribe before reading so a status update in between is not missed.
		ch := waitChan(name)
		ing, err := Get(name)
		if err != nil {
			return nil, err
		}
		if ing.Status.ObservedGeneration >= generation {
			return ing, nil
		}
		select {
		case <-ch:
		case <-ctx.Done():
			return ing, cosy.WrapErrorWithParams(ErrWaitTimeout, name, strconv.FormatInt(generation, 10))
		}
	}
}

// startWorker applies queued changes until ctx ends. It is the only
// goroutine that writes ingress files.
func startWorker(ctx context.Context) {
	enqueueUnapplied()
	ensureCertificates(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-queue.signal:
		}
		if !coalesce(ctx) {
			return
		}
		applies, deletes := queue.take()
		if len(applies) == 0 && len(deletes) == 0 {
			continue
		}
		processBatch(ctx, applies, deletes)
	}
}

// coalesce waits until changes stop arriving for coalesceQuiet, at most
// coalesceMax. It returns false when ctx ended.
func coalesce(ctx context.Context) bool {
	deadline := time.Now().Add(coalesceMax)
	for {
		wait := min(coalesceQuiet, time.Until(deadline))
		if wait <= 0 {
			return true
		}
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return false
		case <-queue.signal:
			timer.Stop()
		case <-timer.C:
			return true
		}
	}
}

// enqueueUnapplied queues the ingresses a previous run left unapplied, for
// example after a restart while the worker was applying.
func enqueueUnapplied() {
	db := model.UseDB()
	if db == nil {
		return
	}
	var all []*model.Ingress
	if err := db.Find(&all).Error; err != nil {
		logger.Errorf("Loading ingresses failed: %v", err)
		return
	}
	for _, ing := range all {
		if ing.Status.ObservedGeneration < ing.Generation || ing.Status.Phase == model.IngressPhaseApplying {
			Enqueue(ing.Name)
		}
	}
}

// ensureCertificates resumes certificate issuance after a restart: it runs
// only in memory, so nothing else would pick up an ACME ingress whose
// certificate was not issued yet. Ingresses queued for apply are left to the
// worker, which calls Ensure after applying them. Ensure returns at once when
// the certificate is complete.
func ensureCertificates(ctx context.Context) {
	db := model.UseDB()
	if db == nil {
		return
	}
	var all []*model.Ingress
	if err := db.Order("name").Find(&all).Error; err != nil {
		logger.Errorf("Loading ingresses failed: %v", err)
		return
	}
	for _, ing := range all {
		if ing.Spec.TLS.Mode != model.IngressTLSModeACME ||
			ing.Status.ObservedGeneration < ing.Generation || ing.Status.Phase == model.IngressPhaseApplying {
			continue
		}
		certs().Ensure(ctx, ing)
	}
}

// processBatch applies and removes one batch of ingresses.
func processBatch(ctx context.Context, applyNames []string, deletes map[string]*deleteJob) {
	var applies, removes []*model.Ingress
	deleteNames := make([]string, 0, len(deletes))
	for name := range deletes {
		deleteNames = append(deleteNames, name)
	}
	sort.Strings(deleteNames)
	for _, name := range deleteNames {
		ing, err := loadIngress(name)
		if err != nil {
			finishDelete(deletes[name], nilIfNotFound(err))
			delete(deletes, name)
			continue
		}
		removes = append(removes, ing)
	}
	for _, name := range applyNames {
		if _, ok := deletes[name]; ok {
			continue
		}
		ing, err := loadIngress(name)
		if err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				logger.Errorf("Loading ingress %s failed: %v", name, err)
			}
			continue
		}
		applies = append(applies, ing)
	}

	for _, ing := range applies {
		_, err := UpdateStatus(ing.Name, func(cur *model.Ingress) {
			cur.Status.Phase = model.IngressPhaseApplying
		})
		if err != nil {
			logger.Errorf("Marking ingress %s as applying failed: %v", ing.Name, err)
		}
	}

	states := make(map[string]CertState, len(applies))
	for _, ing := range applies {
		states[ing.Name] = resolveCert(ctx, ing)
	}

	res := runBisect(ctx, applies, removes, states)

	for _, ing := range applies {
		r := res.applies[ing.Name]
		recordApplyStatus(ing, r, states[ing.Name])
		// Ensure is cheap and idempotent; it also catches a reissue for a
		// host added while the previous certificate is still served.
		if !r.failed && ing.Spec.TLS.Mode == model.IngressTLSModeACME {
			certs().Ensure(ctx, ing)
		}
	}
	for _, item := range res.changed {
		syncToNodes(ctx, item)
	}
	for _, item := range res.removed {
		syncRemoveToNodes(ctx, item)
	}

	for _, ing := range removes {
		r := res.removes[ing.Name]
		job := deletes[ing.Name]
		if r.failed {
			recordDeleteFailure(ing, r)
			finishDelete(job, cosy.WrapErrorWithParams(ErrApplyFailed, r.Error))
			continue
		}
		finishDelete(job, completeDelete(ctx, ing, job.purgeCert))
	}
}

func nilIfNotFound(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	return err
}

func finishDelete(job *deleteJob, err error) {
	if job == nil {
		return
	}
	for _, done := range job.done {
		done <- err
	}
}

// runBisect runs the batch and, when the combined nginx test fails, splits it
// in halves and runs each again, so only the ingresses that break the test
// are marked failed.
func runBisect(ctx context.Context, applies, removes []*model.Ingress, states map[string]CertState) *batchResult {
	res := runBatch(ctx, applies, removes, states, false)
	if !res.txFailed || res.txReason != applyReasonTestFailed || len(applies)+len(removes) <= 1 {
		return res
	}

	type unit struct {
		ing    *model.Ingress
		remove bool
	}
	units := make([]unit, 0, len(applies)+len(removes))
	for _, ing := range removes {
		units = append(units, unit{ing: ing, remove: true})
	}
	for _, ing := range applies {
		units = append(units, unit{ing: ing})
	}
	split := func(part []unit) ([]*model.Ingress, []*model.Ingress) {
		var a, r []*model.Ingress
		for _, u := range part {
			if u.remove {
				r = append(r, u.ing)
			} else {
				a = append(a, u.ing)
			}
		}
		return a, r
	}

	merged := &batchResult{
		applies: make(map[string]*itemResult, len(applies)),
		removes: make(map[string]*itemResult, len(removes)),
	}
	half := len(units) / 2
	for _, part := range [][]unit{units[:half], units[half:]} {
		a, r := split(part)
		sub := runBisect(ctx, a, r, states)
		for name, result := range sub.applies {
			merged.applies[name] = result
		}
		for name, result := range sub.removes {
			merged.removes[name] = result
		}
		merged.changed = append(merged.changed, sub.changed...)
		merged.removed = append(merged.removed, sub.removed...)
	}
	return merged
}
