package nginx

import (
	"github.com/tufanbarisyildirim/gonginx"
	"github.com/tufanbarisyildirim/gonginx/parser"
)

func (c *NgxConfig) FmtCode() (fmtContent string) {
	fmtContent = gonginx.DumpConfig(c.c, gonginx.IndentedStyle)
	return
}

func FmtCode(content string) (fmtContent string) {
	p := parser.NewStringParser(content)
	c := p.Parse()
	fmtContent = gonginx.DumpConfig(c, gonginx.IndentedStyle)
	return
}
