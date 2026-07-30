package clustersync

import (
	"context"
	"runtime"
	"sync"

	"github.com/uozi-tech/cosy/logger"
)

// item is one unit of work replicated to a node.
type item struct {
	kind Kind
	name string
	push func(ctx context.Context, node nodeRef) error
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

			for _, current := range items {
				if ctx.Err() != nil {
					results.fail(node, current.kind, current.name, ctx.Err())
					continue
				}

				if err := current.push(ctx, node); err != nil {
					logger.Errorf("cluster sync %s %s to %s: %v", current.kind, current.name, node.name, err)
					results.fail(node, current.kind, current.name, err)
					continue
				}

				results.ok(node, current.kind, current.name)
			}
		}(node)
	}

	wg.Wait()

	return results.summary()
}
