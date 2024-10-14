package nginx

import (
	"fmt"
	"github.com/tufanbarisyildirim/gonginx/config"
	"github.com/tufanbarisyildirim/gonginx/parser"
	"strings"
	"testing"
)

func TestNgxConfParse(t *testing.T) {
	p, err := parser.NewParser("conf/nextcloud_ngx.conf")
	if err != nil {
		fmt.Println(err)
		return
	}
	n := p.Parse()

	fn(n.Block, 0)

	c, err := ParseNgxConfig("conf/nextcloud_ngx.conf")
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(c)
	c, err = ParseNgxConfig("conf/test.conf")
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(c)
}

func fn(block config.IBlock, deep int) {
	if block == nil {
		return
	}
	for _, v := range block.GetDirectives() {
		if len(v.GetComment()) > 0 {
			for _, c := range v.GetComment() {
				fmt.Println(strings.Repeat("\t", deep), c)
			}
		}

		fmt.Println(fmt.Sprintf("%s%s %s", strings.Repeat("\t", deep), v.GetName(), strings.Join(v.GetParameters(), " ")))
		fn(v.GetBlock(), deep+1)
	}
}
