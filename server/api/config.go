package api

import (
	"github.com/0xJacky/Nginx-UI/server/pkg/config_list"
	"github.com/0xJacky/Nginx-UI/server/pkg/nginx"
	"github.com/0xJacky/Nginx-UI/server/query"
	"github.com/gin-gonic/gin"
	"log"
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

	configFiles, err := os.ReadDir(nginx.GetConfPath(dir))

	if err != nil {
		ErrHandler(c, err)
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
			targetPath, err = os.Readlink(nginx.GetConfPath(file.Name()))
			if err != nil {
				log.Println("GetConfigs Read Symlink Error", targetPath, err)
				continue
			}

			var targetInfo os.FileInfo
			targetInfo, err = os.Stat(targetPath)
			if err != nil {
				log.Println("GetConfigs Stat Error", targetPath, err)
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
	path := nginx.GetConfPath("/", name)

	content, err := os.ReadFile(path)

	if err != nil {
		ErrHandler(c, err)
		return
	}

	g := query.ChatGPTLog
	chatgpt, err := g.Where(g.Name.Eq(path)).FirstOrCreate()

	if err != nil {
		ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"config":           string(content),
		"chatgpt_messages": chatgpt.Content,
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
		ErrHandler(c, err)
		return
	}

	name := request.Name
	content := request.Content

	path := nginx.GetConfPath("/", name)

	if _, err = os.Stat(path); err == nil {
		c.JSON(http.StatusNotAcceptable, gin.H{
			"message": "config exist",
		})
		return
	}

	if content != "" {
		err = os.WriteFile(path, []byte(content), 0644)
		if err != nil {
			ErrHandler(c, err)
			return
		}
	}

	output := nginx.Reload()

	if nginx.GetLogLevel(output) >= nginx.Warn {
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
		ErrHandler(c, err)
		return
	}
	path := nginx.GetConfPath("/", name)
	content := request.Content

	origContent, err := os.ReadFile(path)
	if err != nil {
		ErrHandler(c, err)
		return
	}

	if content != "" && content != string(origContent) {
		// model.CreateBackup(path)
		err = os.WriteFile(path, []byte(content), 0644)
		if err != nil {
			ErrHandler(c, err)
			return
		}
	}

	output := nginx.Reload()

	if nginx.GetLogLevel(output) >= nginx.Warn {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": output,
		})
		return
	}

	GetConfig(c)
}
