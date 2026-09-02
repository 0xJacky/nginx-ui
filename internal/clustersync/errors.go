package clustersync

import "github.com/uozi-tech/cosy"

var (
	e = cosy.NewErrorScope("cluster_sync")
	// ErrPathOutsideConfDir guards directory deployment against traversal.
	ErrPathOutsideConfDir = e.New(50014, "path is not under the nginx conf dir")
	// ErrNoTargetNode is returned when no enabled node matches the request.
	ErrNoTargetNode = e.New(40404, "no enabled target node was found")
	// ErrNamespaceHasNoNode is returned when a namespace has no member node.
	ErrNamespaceHasNoNode = e.New(40405, "the namespace has no node to sync with")
	// ErrEmptyScope is returned when a run would replicate nothing.
	ErrEmptyScope = e.New(40006, "select at least one kind of content to sync")
)
