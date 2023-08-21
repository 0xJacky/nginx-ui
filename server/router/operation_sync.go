package router

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/0xJacky/Nginx-UI/server/internal/analytic"
	"github.com/0xJacky/Nginx-UI/server/internal/logger"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sync"
)

type ErrorRes struct {
	Message string `json:"message"`
}

type toolBodyWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (r toolBodyWriter) Write(b []byte) (int, error) {
	return r.body.Write(b)
}

// OperationSync 针对配置了vip的环境操作进行同步
func OperationSync() gin.HandlerFunc {
	return func(c *gin.Context) {
		bodyBytes, _ := PeekRequest(c.Request)
		wb := &toolBodyWriter{
			body:           &bytes.Buffer{},
			ResponseWriter: c.Writer,
		}
		c.Writer = wb

		c.Next()
		if c.Request.Method == "GET" || !statusValid(c.Writer.Status()) { // 请求有问题，无需执行同步操作
			wb.ResponseWriter.Write(wb.body.Bytes())
			return
		}

		totalCount := 0
		successCount := 0
		detailMsg := ""
		// 后置处理操作同步
		wg := sync.WaitGroup{}
		for _, node := range analytic.NodeMap {
			wg.Add(1)
			go func(data analytic.Node) {
				defer wg.Done()
				if *node.OperationSync && node.Status && requestUrlMatch(c.Request.URL.Path, data) { // 开启操作同步且当前状态正常
					totalCount++
					if err := syncNodeOperation(c, data, bodyBytes); err != nil {
						detailMsg += fmt.Sprintf("node_name: %s, err_msg: %s; ", data.Name, err)
						return
					}
					successCount++
				}
			}(*node)
		}
		wg.Wait()
		if successCount < totalCount { // 如果有错误，替换原来的消息内容
			originBytes := wb.body
			logger.Infof("origin response body: %s", originBytes)
			// clear Origin Buffer
			wb.body = &bytes.Buffer{}
			wb.ResponseWriter.WriteHeader(http.StatusInternalServerError)

			errorRes := ErrorRes{
				Message: fmt.Sprintf("operation sync failed, total: %d, success: %d, fail: %d, detail: %s", totalCount, successCount, totalCount-successCount, detailMsg),
			}
			byts, _ := json.Marshal(errorRes)
			wb.Write(byts)
		}
		wb.ResponseWriter.Write(wb.body.Bytes())
	}
}

func PeekRequest(request *http.Request) ([]byte, error) {
	if request.Body != nil {
		byts, err := io.ReadAll(request.Body) // io.ReadAll as Go 1.16, below please use ioutil.ReadAll
		if err != nil {
			return nil, err
		}
		request.Body = io.NopCloser(bytes.NewReader(byts))
		return byts, nil
	}
	return make([]byte, 0), nil
}

func requestUrlMatch(url string, node analytic.Node) bool {
	p, _ := regexp.Compile(node.SyncApiRegex)
	result := p.FindAllString(url, -1)
	if len(result) > 0 && result[0] == url {
		return true
	}
	return false
}

func statusValid(code int) bool {
	return code < http.StatusMultipleChoices
}

func syncNodeOperation(c *gin.Context, node analytic.Node, bodyBytes []byte) error {
	u, err := url.JoinPath(node.URL, c.Request.RequestURI)
	if err != nil {
		return err
	}
	decodedUri, err := url.QueryUnescape(u)
	if err != nil {
		return err
	}
	logger.Debugf("syncNodeOperation request: %s, node_id: %d, node_name: %s", decodedUri, node.ID, node.Name)
	client := http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	req, err := http.NewRequest(c.Request.Method, decodedUri, bytes.NewReader(bodyBytes))
	req.Header.Set("X-Node-Secret", node.Token)

	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	byts, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}
	if !statusValid(res.StatusCode) {
		errRes := ErrorRes{}
		if err = json.Unmarshal(byts, &errRes); err != nil {
			return err
		}
		return errors.New(errRes.Message)
	}
	logger.Debug("syncNodeOperation result: ", string(byts))
	return nil
}
