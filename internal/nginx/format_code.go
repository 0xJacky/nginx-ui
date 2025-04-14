package nginx

import (
	"github.com/tufanbarisyildirim/gonginx/dumper"
	"github.com/tufanbarisyildirim/gonginx/parser"
	"github.com/uozi-tech/cosy/logger"
)

func (c *NgxConfig) FmtCode() (fmtContent string) {
	fmtContent = dumper.DumpConfig(c.c, dumper.IndentedStyle)
	return
}

func FmtCode(content string) (fmtContent string, err error) {
	logger.Debugf("content: %s", content)
	p := parser.NewStringParser(content, parser.WithSkipValidDirectivesErr())
	c, err := p.Parse()
	if err != nil {
		return
	}
	fmtContent = dumper.DumpConfig(c, dumper.IndentedStyle)
	return
}
