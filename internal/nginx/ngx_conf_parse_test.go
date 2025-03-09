package nginx

import (
	"fmt"
	"strings"
	"testing"

	"github.com/tufanbarisyildirim/gonginx/config"
	"github.com/tufanbarisyildirim/gonginx/parser"
)

func TestNgxConfParse(t *testing.T) {
	p, err := parser.NewParser("conf/nextcloud_ngx.conf")
	if err != nil {
		fmt.Println(err)
		return
	}

	n, _ := p.Parse()

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

		// 将 []config.Parameter 转换为 []string
		params := make([]string, len(v.GetParameters()))
		for i, p := range v.GetParameters() {
			params[i] = p.String()
		}
		fmt.Printf("%s%s %s\n", strings.Repeat("\t", deep), v.GetName(), strings.Join(params, " "))
		fn(v.GetBlock(), deep+1)
	}
}
