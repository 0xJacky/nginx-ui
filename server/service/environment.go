package service

import (
	"crypto/tls"
	"encoding/json"
	"github.com/0xJacky/Nginx-UI/server/internal/logger"
	"github.com/0xJacky/Nginx-UI/server/model"
	"github.com/0xJacky/Nginx-UI/server/query"
	"io"
	"net/http"
	"net/url"
	"time"
)

type Environment struct {
	*model.Environment
	Status bool `json:"status"`
	NodeInfo
}

func RetrieveEnvironmentList() (envs []*Environment, err error) {
	envQuery := query.Environment

	data, err := envQuery.Find()
	if err != nil {
		return
	}

	for _, v := range data {
		t := &Environment{
			Environment: v,
		}

		node, status := t.GetNode()
		t.Status = status
		t.NodeInfo = node

		envs = append(envs, t)
	}

	return
}

type NodeInfo struct {
	RequestNodeSecret string      `json:"request_node_secret"`
	NodeRuntimeInfo   RuntimeInfo `json:"node_runtime_info"`
	Version           string      `json:"version"`
	CPUNum            int         `json:"cpu_num"`
	MemoryTotal       string      `json:"memory_total"`
	ResponseAt        time.Time   `json:"response_at"`
}

func (env *Environment) GetNode() (node NodeInfo, status bool) {
	u, err := url.JoinPath(env.URL, "/api/node")

	if err != nil {
		logger.Error(err)
		return
	}
	client := http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	req, err := http.NewRequest("GET", u, nil)
	req.Header.Set("X-Node-Secret", env.Token)

	resp, err := client.Do(req)

	if err != nil {
		logger.Error(err)
		return
	}

	defer resp.Body.Close()
	bytes, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		logger.Error(string(bytes))
		return
	}

	logger.Debug(string(bytes))
	err = json.Unmarshal(bytes, &node)
	if err != nil {
		logger.Error(err)
		return
	}

	node.ResponseAt = time.Now()
	status = true

	return
}
