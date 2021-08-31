package api

import (
    "github.com/0xJacky/Nginx-UI/server/model"
    "github.com/gin-gonic/gin"
    "golang.org/x/crypto/bcrypt"
    "log"
    "net/http"
)

type LoginUser struct {
    Name     string `json:"name" binding:"required,max=255"`
    Password string `json:"password" binding:"required,max=255"`
}

func Login(c *gin.Context) {
    var user LoginUser
    ok, verrs := BindAndValid(c, &user)
    if !ok {
        c.JSON(http.StatusNotAcceptable, gin.H{
            "errors": verrs,
        })
        return
    }

    u, err := model.GetUser(user.Name)
    if err != nil {
        log.Println(err)
        c.JSON(http.StatusForbidden, gin.H{
            "message": "Incorrect name or password",
        })
        return
    }

    if err = bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(user.Password)); err != nil {
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
    if token != "" {
        err := model.DeleteToken(token)
        if err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{
                "message": err.Error(),
            })
            return
        }
    }
    c.JSON(http.StatusNoContent, gin.H{})
}
