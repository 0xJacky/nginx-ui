package api

import (
    "github.com/gin-gonic/gin"
    "io/ioutil"
    "net/http"
    "os"
    "path/filepath"
)

func GetTemplate(c *gin.Context)  {
    name := c.Param("name")
    path := filepath.Join("template", name)
    content, err := ioutil.ReadFile(path)

    if err != nil {
        if os.IsNotExist(err) {
            c.JSON(http.StatusNotFound, gin.H{
                "message": err.Error(),
            })
            return
        }
        ErrorHandler(c, err)
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "message": "ok",
        "template": string(content),
    })
}
