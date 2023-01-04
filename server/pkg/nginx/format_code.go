package nginx

import (
	"github.com/tufanbarisyildirim/gonginx"
)

func (c *NgxConfig) FmtCode() (fmtContent string) {
	fmtContent = gonginx.DumpConfig(c.c, gonginx.IndentedStyle)
	return
}
