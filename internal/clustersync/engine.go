package clustersync

import (
	"context"
	"fmt"
	"runtime"
	"sync"

	"github.com/uozi-tech/cosy/logger"
)

// item is one unit of work replicated to a node.
type item struct {
	kind     Kind
	name     string
	blocking bool
	push     func(ctx context.Context, node nodeRef) error
	// pushOutcome is an alternative to push for items that learn something from
	// a successful answer, such as files the node left alone.
	pushOutcome func(ctx context.Context, node nodeRef) (outcome, error)
}

// outcome carries what a node reported about a successful push.
type outcome struct {
	skippedExisting int
	skippedPaths    []string
}

func (i item) execute(ctx context.Context, node nodeRef) (outcome, error) {
	if i.pushOutcome != nil {
		return i.pushOutcome(ctx, node)
	}

	return outcome{}, i.push(ctx, node)
}

// run pushes every item to every node. Nodes are processed concurrently while a
// single node receives its items sequentially, which keeps the remote reload
// order predictable and avoids hammering a node with parallel writes.
func run(ctx context.Context, nodes []nodeRef, items []item) *Summary {
	results := &collector{}
	if len(nodes) == 0 || len(items) == 0 {
		return results.summary()
	}

	wg := &sync.WaitGroup{}
	wg.Add(len(nodes))

	for _, node := range nodes {
		go func(node nodeRef) {
			defer func() {
				if err := recover(); err != nil {
					buf := make([]byte, 1024)
					runtime.Stack(buf, false)
					logger.Errorf("%s\n%s", err, buf)
				}
			}()
			defer wg.Done()

			var blockedBy error
			for _, current := range items {
				if ctx.Err() != nil {
					results.fail(node, current.kind, current.name, ctx.Err())
					continue
				}
				if blockedBy != nil {
					results.fail(node, current.kind, current.name, fmt.Errorf("skipped after prerequisite failed: %w", blockedBy))
					continue
				}

				pushed, err := current.execute(ctx, node)
				if err != nil {
					logger.Errorf("cluster sync %s %s to %s: %v", current.kind, current.name, node.name, err)
					results.fail(node, current.kind, current.name, err)
					if current.blocking {
						blockedBy = fmt.Errorf("%s %s: %w", current.kind, current.name, err)
					}
					continue
				}

				results.okWith(node, current.kind, current.name, pushed)
			}
		}(node)
	}

	wg.Wait()

	return results.summary()
}
