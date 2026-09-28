package access_list

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newControlRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.AccessList{}))
	model.Use(db)
	query.SetDefault(db)
	t.Cleanup(func() { model.Use(nil) })

	require.NoError(t, db.Create(&model.AccessList{Name: "LAN", Slug: "lan", Fallback: model.AccessFallbackDeny}).Error)

	router := gin.New()
	group := router.Group("/")
	group.POST("access_control/state", GetAccessControlState)
	group.POST("access_control/apply", ApplyAccessControl)
	return router
}

func postJSON(router http.Handler, path string, body any) *httptest.ResponseRecorder {
	payload, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

const controlSite = "server {\n    listen 80;\n    server_name a.example.com;\n    location / {\n        return 200;\n    }\n}\n"

func TestApplyAccessControlText(t *testing.T) {
	router := newControlRouter(t)

	w := postJSON(router, "/access_control/apply", gin.H{
		"content": controlSite,
		"changes": []gin.H{{"server": 0, "mode": "list", "slug": "lan"}},
	})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var resp struct {
		Content string `json:"content"`
		Servers []struct {
			Mode string `json:"mode"`
			Slug string `json:"slug"`
		} `json:"servers"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Contains(t, resp.Content, "    server_name a.example.com;\n    include nginx-ui/access/lan.conf;\n")
	require.Len(t, resp.Servers, 1)
	assert.Equal(t, "list", resp.Servers[0].Mode)
	assert.Equal(t, "lan", resp.Servers[0].Slug)
}

func TestApplyAccessControlStructured(t *testing.T) {
	router := newControlRouter(t)

	w := postJSON(router, "/access_control/apply", gin.H{
		"config": gin.H{
			"servers": []gin.H{{
				"directives": []gin.H{{"directive": "listen", "params": "80", "idx": 0}},
				"locations":  []gin.H{{"path": "/", "content": "return 200;\n"}},
			}},
		},
		"changes": []gin.H{{"server": 0, "location": 0, "mode": "public"}},
	})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var resp struct {
		Config struct {
			Servers []struct {
				Locations []struct {
					Content string `json:"content"`
				} `json:"locations"`
			} `json:"servers"`
		} `json:"config"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "allow all;\nreturn 200;\n", resp.Config.Servers[0].Locations[0].Content)
}

func TestApplyAccessControlRejectsUnknownList(t *testing.T) {
	router := newControlRouter(t)

	w := postJSON(router, "/access_control/apply", gin.H{
		"content": controlSite,
		"changes": []gin.H{{"server": 0, "mode": "list", "slug": "office"}},
	})
	assert.NotEqual(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "office")
}

func TestAccessControlStateIgnoresBrokenContent(t *testing.T) {
	router := newControlRouter(t)

	w := postJSON(router, "/access_control/state", gin.H{"content": "server {"})
	assert.NotEqual(t, http.StatusOK, w.Code)

	w = postJSON(router, "/access_control/state", gin.H{"content": controlSite})
	require.Equal(t, http.StatusOK, w.Code)
	assert.NotContains(t, w.Body.String(), "\"content\"", "state does not echo the content back")
}
