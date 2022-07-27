package nginx

import (
	"github.com/emirpasic/gods/queues/linkedlistqueue"
	"strings"
)

type CommentQueue struct {
	*linkedlistqueue.Queue
}

type NgxConfig struct {
	FileName     string         `json:"file_name"`
	Upstreams    []*NgxUpstream `json:"upstreams"`
	Servers      []*NgxServer   `json:"servers"`
	Custom       string         `json:"custom"`
	commentQueue *CommentQueue
}

type NgxServer struct {
	Directives   []*NgxDirective `json:"directives"`
	Locations    []*NgxLocation  `json:"locations"`
	Comments     string          `json:"comments"`
	commentQueue *CommentQueue
}

type NgxUpstream struct {
	Name       string          `json:"name"`
	Directives []*NgxDirective `json:"directives"`
	Comments   string          `json:"comments"`
}

type NgxDirective struct {
	Directive string `json:"directive"`
	Params    string `json:"params"`
	Comments  string `json:"comments"`
}

type NgxDirectives map[string][]NgxDirective

type NgxLocation struct {
	Path     string `json:"path"`
	Content  string `json:"content"`
	Comments string `json:"comments"`
}

func (c *CommentQueue) DequeueAllComments() (comments string) {
	for !c.Empty() {
		comment, ok := c.Dequeue()

		if ok {
			comments += strings.TrimSpace(comment.(string)) + "\n"
		}
	}

	return
}

func (d *NgxDirective) Orig() string {
	return d.Directive + " " + d.Params
}

func (d *NgxDirective) TrimParams() {
	d.Params = strings.TrimRight(strings.TrimSpace(d.Params), ";")
	return
}

func NewNgxServer() *NgxServer {
	return &NgxServer{commentQueue: &CommentQueue{linkedlistqueue.New()}}
}

func NewNgxConfig(filename string) *NgxConfig {
	return &NgxConfig{FileName: filename, commentQueue: &CommentQueue{linkedlistqueue.New()}}
}
