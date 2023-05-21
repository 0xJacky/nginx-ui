package api

import (
    "github.com/0xJacky/Nginx-UI/server/internal/logger"
    "github.com/0xJacky/Nginx-UI/server/internal/upgrader"
    "github.com/0xJacky/Nginx-UI/server/settings"
    "github.com/gin-gonic/gin"
    "github.com/gorilla/websocket"
    "net/http"
    "os"
)

func GetRelease(c *gin.Context) {
    data, err := upgrader.GetRelease(c.Query("channel"))
    if err != nil {
        ErrHandler(c, err)
        return
    }
    runtimeInfo, err := upgrader.GetRuntimeInfo()
    if err != nil {
        ErrHandler(c, err)
        return
    }
    type resp struct {
        upgrader.TRelease
        upgrader.RuntimeInfo
    }
    c.JSON(http.StatusOK, resp{
        data, runtimeInfo,
    })
}

func GetCurrentVersion(c *gin.Context) {
    curVer, err := upgrader.GetCurrentVersion()
    if err != nil {
        ErrHandler(c, err)
        return
    }

    c.JSON(http.StatusOK, curVer)
}

func PerformCoreUpgrade(c *gin.Context) {
    var upGrader = websocket.Upgrader{
        CheckOrigin: func(r *http.Request) bool {
            return true
        },
    }
    // upgrade http to websocket
    ws, err := upGrader.Upgrade(c.Writer, c.Request, nil)
    if err != nil {
        logger.Error(err)
        return
    }
    defer ws.Close()

    var control struct {
        DryRun  bool   `json:"dry_run"`
        Channel string `json:"channel"`
    }

    err = ws.ReadJSON(&control)

    if err != nil {
        logger.Error(err)
        return
    }

    _ = ws.WriteJSON(gin.H{
        "status":  "info",
        "message": "Initialing core upgrader",
    })

    u, err := upgrader.NewUpgrader(control.Channel)

    if err != nil {
        _ = ws.WriteJSON(gin.H{
            "status":  "error",
            "message": "Initial core upgrader error",
        })
        _ = ws.WriteJSON(gin.H{
            "status":  "error",
            "message": err.Error(),
        })
        logger.Error(err)
        return
    }
    _ = ws.WriteJSON(gin.H{
        "status":  "info",
        "message": "Downloading latest release",
    })
    progressChan := make(chan float64)
    go func() {
        for progress := range progressChan {
            _ = ws.WriteJSON(gin.H{
                "status":   "progress",
                "progress": progress,
            })
        }
    }()

    tarName, err := u.DownloadLatestRelease(progressChan)

    if err != nil {
        _ = ws.WriteJSON(gin.H{
            "status":  "error",
            "message": "Download latest release error",
        })
        _ = ws.WriteJSON(gin.H{
            "status":  "error",
            "message": err.Error(),
        })
        logger.Error(err)
        return
    }

    defer func() {
        _ = os.Remove(tarName)
        _ = os.Remove(tarName + ".digest")
    }()
    _ = ws.WriteJSON(gin.H{
        "status":  "info",
        "message": "Performing core upgrade",
    })
    // dry run
    if control.DryRun || settings.ServerSettings.Demo {
        return
    }

    _ = os.Remove(u.ExPath)
    // bye, overseer will restart nginx-ui
    err = u.PerformCoreUpgrade(u.ExPath, tarName)
    if err != nil {
        _ = ws.WriteJSON(gin.H{
            "status":  "error",
            "message": "Perform core upgrade error",
        })
        _ = ws.WriteJSON(gin.H{
            "status":  "error",
            "message": err.Error(),
        })
        logger.Error(err)
        return
    }
}
