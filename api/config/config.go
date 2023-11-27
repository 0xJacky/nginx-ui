package config

import (
	"github.com/0xJacky/Nginx-UI/api"
	"github.com/0xJacky/Nginx-UI/internal/config_list"
	"github.com/0xJacky/Nginx-UI/internal/logger"
	nginx2 "github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gin-gonic/gin"
	"github.com/sashabaranov/go-openai"
	"net/http"
	"os"
)

func GetConfigs(c *gin.Context) {
	orderBy := c.Query("order_by")
	sort := c.DefaultQuery("sort", "desc")
	dir := c.DefaultQuery("dir", "/")

	mySort := map[string]string{
		"name":   "string",
		"modify": "time",
		"is_dir": "bool",
	}

	configFiles, err := os.ReadDir(nginx2.GetConfPath(dir))

	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	var configs []gin.H

	for i := range configFiles {
		file := configFiles[i]
		fileInfo, _ := file.Info()

		switch mode := fileInfo.Mode(); {
		case mode.IsRegular(): // regular file, not a hidden file
			if "." == file.Name()[0:1] {
				continue
			}
		case mode&os.ModeSymlink != 0: // is a symbol
			var targetPath string
			targetPath, err = os.Readlink(nginx2.GetConfPath(file.Name()))
			if err != nil {
				logger.Error("Read Symlink Error", targetPath, err)
				continue
			}

			var targetInfo os.FileInfo
			targetInfo, err = os.Stat(targetPath)
			if err != nil {
				logger.Error("Stat Error", targetPath, err)
				continue
			}
			// but target file is not a dir
			if targetInfo.IsDir() {
				continue
			}
		}

		configs = append(configs, gin.H{
			"name":   file.Name(),
			"size":   fileInfo.Size(),
			"modify": fileInfo.ModTime(),
			"is_dir": file.IsDir(),
		})
	}

	configs = config_list.Sort(orderBy, sort, mySort[orderBy], configs)

	c.JSON(http.StatusOK, gin.H{
		"data": configs,
	})
}

func GetConfig(c *gin.Context) {
	name := c.Param("name")
	path := nginx2.GetConfPath("/", name)

	stat, err := os.Stat(path)

	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	content, err := os.ReadFile(path)

	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	g := query.ChatGPTLog
	chatgpt, err := g.Where(g.Name.Eq(path)).FirstOrCreate()

	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	if chatgpt.Content == nil {
		chatgpt.Content = make([]openai.ChatCompletionMessage, 0)
	}

	c.JSON(http.StatusOK, gin.H{
		"config":           string(content),
		"chatgpt_messages": chatgpt.Content,
		"file_path":        path,
		"modified_at":      stat.ModTime(),
	})

}

type AddConfigJson struct {
	Name    string `json:"name" binding:"required"`
	Content string `json:"content" binding:"required"`
}

func AddConfig(c *gin.Context) {
	var request AddConfigJson
	err := c.BindJSON(&request)
	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	name := request.Name
	content := request.Content

	path := nginx2.GetConfPath("/", name)

	if _, err = os.Stat(path); err == nil {
		c.JSON(http.StatusNotAcceptable, gin.H{
			"message": "config exist",
		})
		return
	}

	if content != "" {
		err = os.WriteFile(path, []byte(content), 0644)
		if err != nil {
			api.ErrHandler(c, err)
			return
		}
	}

	output := nginx2.Reload()
	if nginx2.GetLogLevel(output) >= nginx2.Warn {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": output,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"name":    name,
		"content": content,
	})

}

type EditConfigJson struct {
	Content string `json:"content" binding:"required"`
}

func EditConfig(c *gin.Context) {
	name := c.Param("name")
	var request EditConfigJson
	err := c.BindJSON(&request)
	if err != nil {
		api.ErrHandler(c, err)
		return
	}
	path := nginx2.GetConfPath("/", name)
	content := request.Content

	origContent, err := os.ReadFile(path)
	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	if content != "" && content != string(origContent) {
		// model.CreateBackup(path)
		err = os.WriteFile(path, []byte(content), 0644)
		if err != nil {
			api.ErrHandler(c, err)
			return
		}
	}

	output := nginx2.Reload()

	if nginx2.GetLogLevel(output) >= nginx2.Warn {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": output,
		})
		return
	}

	GetConfig(c)
}
