package convert

import "github.com/uozi-tech/cosy"

var (
	e                       = cosy.NewErrorScope("upstream")
	ErrInvalidRequest       = e.New(40017, "a site name and an upstream name are required")
	ErrNestedBlock          = e.New(40018, "upstream {0} contains a nested {1} block, which an upstream group cannot hold")
	ErrMultilineValue       = e.New(40019, "the {0} directive of upstream {1} has a value that spans several lines, which an upstream group cannot hold")
	ErrCommentWithBraces    = e.New(40020, "a comment inside upstream {0} contains braces, which an upstream group cannot hold; edit the comment first")
	ErrUnsupportedContext   = e.New(40021, "upstream {0} is defined inside a {1} block; only upstream blocks of the http context can become an upstream group")
	ErrInvalidSiteName      = e.New(40022, "invalid site name: {0}")
	ErrSiteNotFound         = e.New(40404, "site not found: {0}")
	ErrGroupExists          = e.New(40906, "an upstream group named {0} already exists; rename this upstream before converting it")
	ErrUpstreamDefinedTwice = e.New(40907, "upstream {0} is defined more than once in {1}")
)
