package test

import (
	"fmt"
	"github.com/0xJacky/Nginx-UI/server/pkg/nginx"
	"testing"
)

func TestNgxConfParse(t *testing.T) {
	c, err := nginx.ParseNgxConfig("nextcloud_ngx.conf")

	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(c.FileName)
	// directive in root
	fmt.Println("Upstream")
	for _, u := range c.Upstreams {
		fmt.Println("upstream name", u.Name)
		fmt.Printf("comments\n%v", u.Comments)
		for _, d := range u.Directives {
			fmt.Println("u.Directives.d", d)
		}
	}
	fmt.Println("==========================")
	fmt.Println("Servers")
	for _, s := range c.Servers {
		fmt.Printf("comments\n%v", s.Comments)
		for _, d := range s.Directives {
			fmt.Println(d)
		}
		// locations
		for _, location := range s.Locations {
			fmt.Printf("comments\n%v", location.Comments)
			fmt.Println("path", location.Path)
			fmt.Println("content", location.Content)
			fmt.Println("==========================")
		}
	}

}
