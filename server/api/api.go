package api

import (
	"log"
)

func ErrorHandler(err error)  {
	log.Println(err)
}
