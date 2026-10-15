package ingress

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
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
	// generation is the generation the delete was requested at. The row is
	// only deleted while it still has it.
	generation int64
	// superseded is set, under the queue lock, when a write to the ingress
	// arrived after the worker picked the delete up: the write wins.
	superseded bool
	done       []chan error
}

// workQueue holds the changes the worker has not picked up yet.
type workQueue struct {
	mu sync.Mutex
	// applies maps the names to apply to the sequence number of their latest
	// Enqueue.
	applies map[string]uint64
	deletes map[string]*deleteJob
	// inflight holds the deletes of the batch the worker is processing, and
	// inflightSeqs the sequence numbers of its applies.
	inflight     map[string]*deleteJob
	inflightSeqs map[string]uint64
	// seq numbers the Enqueue calls; applied holds, per name, the sequence
	// number of the latest Enqueue the worker finished. Both live in memory:
	// they only tell a waiting request whether its own apply ran.
	seq     uint64
	applied map[string]uint64
	signal  chan struct{}
}

var queue = &workQueue{
	applies:      make(map[string]uint64),
	deletes:      make(map[string]*deleteJob),
	inflight:     make(map[string]*deleteJob),
	inflightSeqs: make(map[string]uint64),
	applied:      make(map[string]uint64),
	signal:       make(chan struct{}, 1),
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
	q.inflightSeqs = q.applies
	q.applies = make(map[string]uint64)
	q.deletes = make(map[string]*deleteJob)
	q.inflight = deletes
	return applies, deletes
}

// done records the applies of the batch the worker finished, forgets its
// deletes and wakes everyone waiting on them.
func (q *workQueue) done() {
	q.mu.Lock()
	names := make([]string, 0, len(q.inflightSeqs)+len(q.inflight))
	for name, seq := range q.inflightSeqs {
		if seq > q.applied[name] {
			q.applied[name] = seq
		}
		names = append(names, name)
	}
	for name := range q.inflight {
		names = append(names, name)
	}
	q.inflightSeqs = make(map[string]uint64)
	q.inflight = make(map[string]*deleteJob)
	q.mu.Unlock()
	for _, name := range names {
		notifyWaiters(name)
	}
}

// forgetApplied drops the apply sequence of a deleted ingress.
func forgetApplied(name string) {
	queue.mu.Lock()
	defer queue.mu.Unlock()
	delete(queue.applied, name)
}

// appliedAfter reports whether the worker finished an apply of name that was
// enqueued after seq.
func appliedAfter(name string, seq uint64) bool {
	queue.mu.Lock()
	defer queue.mu.Unlock()
	return queue.applied[name] > seq
}

// deletePending reports whether a delete of name is queued or running.
func deletePending(name string) bool {
	queue.mu.Lock()
	defer queue.mu.Unlock()
	_, queued := queue.deletes[name]
	_, running := queue.inflight[name]
	return queued || running
}

// ApplySeq returns the sequence number of the latest Enqueue. A request
// captures it before its write and passes it to WaitForApply, which then
// waits for the apply that write triggered, even when the spec, and so the
// generation, did not change.
func ApplySeq() uint64 {
	queue.mu.Lock()
	defer queue.mu.Unlock()
	return queue.seq
}

// supersedeDelete lets a write to name win over a delete requested before
// it: a delete still queued is dropped and its waiters get
// ErrDeleteConflict; a delete the worker is processing keeps the row (see
// completeDelete). Callers hold storeMu, after storing the write.
func supersedeDelete(name string) {
	queue.mu.Lock()
	job, queued := queue.deletes[name]
	if queued {
		delete(queue.deletes, name)
	}
	if inflight, ok := queue.inflight[name]; ok {
		inflight.superseded = true
	}
	queue.mu.Unlock()
	if queued {
		finishDelete(job, cosy.WrapErrorWithParams(ErrDeleteConflict, name))
	}
}

// deleteSuperseded reports whether a write arrived after the worker picked up
// the delete.
func deleteSuperseded(job *deleteJob) bool {
	queue.mu.Lock()
	defer queue.mu.Unlock()
	return job != nil && job.superseded
}

// Enqueue asks the worker to apply the named ingresses. Calls that arrive
// close together are coalesced into one apply.
func Enqueue(names ...string) {
	if len(names) == 0 {
		return
	}
	queue.mu.Lock()
	queue.seq++
	for _, name := range names {
		queue.applies[name] = queue.seq
	}
	queue.mu.Unlock()
	queue.notify()
}

// enqueueDelete asks the worker to remove an ingress and its files, as long
// as it is still at generation. The returned channel receives the outcome
// once. Callers hold storeMu so no write slips in between reading the
// generation and queueing the delete.
func enqueueDelete(name string, purgeCert bool, generation int64) <-chan error {
	done := make(chan error, 1)
	queue.mu.Lock()
	job, ok := queue.deletes[name]
	if !ok {
		job = &deleteJob{}
		queue.deletes[name] = job
	}
	job.purgeCert = job.purgeCert || purgeCert
	job.generation = generation
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
	return waitIngress(ctx, name, generation, nil)
}

// WaitForApply is WaitForGeneration that also waits for an apply enqueued
// after seq (see ApplySeq) to finish, so a request that re-applies an
// unchanged spec does not return the status of an earlier apply.
func WaitForApply(ctx context.Context, name string, generation int64, seq uint64) (*model.Ingress, error) {
	return waitIngress(ctx, name, generation, &seq)
}

func waitIngress(ctx context.Context, name string, generation int64, seq *uint64) (*model.Ingress, error) {
	for {
		// Subscribe before reading so a status update in between is not missed.
		ch := waitChan(name)
		ing, err := Get(name)
		if err != nil {
			return nil, err
		}
		if ing.Status.ObservedGeneration >= generation && (seq == nil || appliedAfter(name, *seq)) {
			return ing, nil
		}
		select {
		case <-ch:
		case <-ctx.Done():
			return ing, cosy.WrapErrorWithParams(ErrWaitTimeout, name, strconv.FormatInt(generation, 10))
		}
	}
}

// WaitForDeletion blocks until no delete of the ingress is queued or running,
// or ctx ends. A delete that lost to a later write leaves the ingress in
// place; that is not an error here.
func WaitForDeletion(ctx context.Context, name string) error {
	for {
		ch := waitChan(name)
		if !deletePending(name) {
			return nil
		}
		select {
		case <-ch:
		case <-ctx.Done():
			return cosy.WrapErrorWithParams(ErrDeleteWaitTimeout, name)
		}
	}
}

// startWorker applies queued changes until ctx ends. It is the only
// goroutine that writes ingress files.
func startWorker(ctx context.Context) {
	defer healthRefreshes.Wait()
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
			queue.done()
			continue
		}
		processBatchSafely(ctx, applies, deletes)
		queue.done()
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

// processBatchSafely runs processBatch and turns a panic into a failed
// batch, so the worker survives it: every apply of the batch gets
// ConfigApplied=false (RenderFailed) with the panic message and every delete
// fails with ErrApplyFailed.
func processBatchSafely(ctx context.Context, applies []string, deletes map[string]*deleteJob) {
	defer func() {
		r := recover()
		if r == nil {
			return
		}
		logger.Errorf("Ingress apply panicked: %v\n%s", r, debug.Stack())
		message := fmt.Sprintf("apply panicked: %v", r)
		for _, name := range applies {
			recordPanic(name, message)
		}
		for _, job := range deletes {
			finishDelete(job, cosy.WrapErrorWithParams(ErrApplyFailed, message))
		}
	}()
	processBatch(ctx, applies, deletes)
}

// recordPanic marks the current generation of an ingress as failed.
func recordPanic(name, message string) {
	_, err := UpdateStatus(name, func(cur *model.Ingress) {
		cur.Status.Phase = ""
		cur.Status.ObservedGeneration = max(cur.Status.ObservedGeneration, cur.Generation)
		SetCondition(&cur.Status, model.IngressCondition{
			Type:    model.IngressConditionConfigApplied,
			Status:  false,
			Reason:  applyReasonRenderFailed,
			Message: truncateMessage(message),
		})
	})
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		logger.Errorf("Recording the failed apply of ingress %s failed: %v", name, err)
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

	var applied []string
	for _, ing := range applies {
		r := res.applies[ing.Name]
		recordApplyStatus(ing, r, states[ing.Name])
		if r.failed {
			continue
		}
		applied = append(applied, ing.Name)
		// Ensure is cheap and idempotent; it also catches a reissue for a
		// host added while the previous certificate is still served.
		if ing.Spec.TLS.Mode == model.IngressTLSModeACME {
			certs().Ensure(ctx, ing)
		}
	}
	if len(applied) > 0 {
		refreshHealthAfterApply(applied)
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
		finishDelete(job, completeDelete(ctx, ing, job))
	}
}

func nilIfNotFound(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	return err
}

// finishDelete hands the outcome to everyone waiting for the delete, once.
func finishDelete(job *deleteJob, err error) {
	if job == nil {
		return
	}
	for _, done := range job.done {
		done <- err
	}
	job.done = nil
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
