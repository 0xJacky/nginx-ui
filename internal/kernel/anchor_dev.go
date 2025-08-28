//go:build dev

package kernel

import (
	"log"
	"net/http"
	_ "net/http/pprof"
)

func Anchor() {
	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()
}
