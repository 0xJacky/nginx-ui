package nginx

import (
	"github.com/tufanbarisyildirim/gonginx"
	"github.com/tufanbarisyildirim/gonginx/parser"
)

func (c *NgxConfig) FmtCode() (fmtContent string) {
	fmtContent = gonginx.DumpConfig(c.c, gonginx.IndentedStyle)
	return
}

func FmtCode(content string) (fmtContent string, err error) {
	p := parser.NewStringParser(content)
	c, err := p.Parse()
	if err != nil {
		return
	}
	fmtContent = gonginx.DumpConfig(c, gonginx.IndentedStyle)
	return
}
