package environment

import (
	"github.com/0xJacky/Nginx-UI/internal/analytic"
	"github.com/0xJacky/Nginx-UI/query"
)

func RetrieveEnvironmentList() (envs []*analytic.Node, err error) {
	envQuery := query.Environment

	data, err := envQuery.Find()
	if err != nil {
		return
	}

	for _, v := range data {
		t := analytic.GetNode(v)

		envs = append(envs, t)
	}

	return
}
