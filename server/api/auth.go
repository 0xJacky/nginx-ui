package api

import (
    "crypto/md5"
    "fmt"
    "github.com/0xJacky/Nginx-UI/model"
    "github.com/gin-gonic/gin"
    "log"
    "net/http"
)

type LoginUser struct {
    Name     string `json:"name"`
    Password string `json:"password"`
}

func Login(c *gin.Context) {
    var user LoginUser
    err := c.BindJSON(&user)
    if err != nil {
        log.Println(err)
    }
    var u model.Auth
    u, err = model.GetUser(user.Name)
    if err != nil {
        log.Println(err)
    }
    data := []byte(user.Password)
    has := md5.Sum(data)
    md5str := fmt.Sprintf("%x", has) // 将[]byte转成16进制
    if u.Password != md5str {
        c.JSON(http.StatusForbidden, gin.H{
            "message": "Incorrect name or password",
        })
        return
    }
    var token string
    token, err = model.GenerateJWT(u.Name)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{
            "message": err.Error(),
        })
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "message": "ok",
        "token": token,
    })
}

func Logout(c *gin.Context) {
    token := c.GetHeader("Authorization")
    err := model.DeleteToken(token)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{
            "message": err.Error(),
        })
        return
    }
    c.JSON(http.StatusNoContent, gin.H{})
}
