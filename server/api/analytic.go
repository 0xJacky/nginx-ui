package api

import (
    "encoding/json"
    "fmt"
    "github.com/0xJacky/Nginx-UI/tool"
    "github.com/dustin/go-humanize"
    "github.com/gin-gonic/gin"
    "github.com/gorilla/websocket"
    "github.com/mackerelio/go-osstat/cpu"
    "github.com/mackerelio/go-osstat/memory"
    "github.com/mackerelio/go-osstat/uptime"
    "github.com/mackerelio/go-osstat/loadavg"
    "net/http"
    "strconv"
    "time"
)

var upGrader = websocket.Upgrader{
    CheckOrigin: func(r *http.Request) bool {
        return true
    },
}

func Analytic(c *gin.Context) {
    // upgrade http to websocket
    ws, err := upGrader.Upgrade(c.Writer, c.Request, nil)
    if err != nil {
        return
    }

    defer ws.Close()

    for {
        // read
        mt, message, err := ws.ReadMessage()
        if err != nil {
            break
        }
        if string(message) == "ping" {

            response := make(gin.H)

            memoryStat, err := memory.Get()
            if err != nil {
                fmt.Println(err)
                return
            }
            response["memory_total"] = humanize.Bytes(memoryStat.Total)
            response["memory_used"] = humanize.Bytes(memoryStat.Used)
            response["memory_cached"] = humanize.Bytes(memoryStat.Cached)
            response["memory_free"] = humanize.Bytes(memoryStat.Free)

            response["memory_pressure"] = memoryStat.Used * 100 / memoryStat.Total

            before, err := cpu.Get()
            if err != nil {
                fmt.Println(err)
            }
            time.Sleep(time.Duration(1) * time.Second)
            after, err := cpu.Get()
            if err != nil {
                fmt.Println(err)
            }

            total := float64(after.Total - before.Total)

            response["cpu_user"], _ = strconv.ParseFloat(fmt.Sprintf("%.2f",
                float64(after.User-before.User)/total*100), 64)

            response["cpu_system"], _ = strconv.ParseFloat(fmt.Sprintf("%.2f",
                float64(after.System-before.System)/total*100), 64)

            response["cpu_idle"], _ = strconv.ParseFloat(fmt.Sprintf("%.2f",
                float64(after.Idle-before.Idle)/total*100), 64)

            response["uptime"], _ = uptime.Get()
            response["uptime"] = response["uptime"].(time.Duration) / time.Second
            response["loadavg"], _ = loadavg.Get()

            used, _total, percentage, err := tool.DiskUsage(".")

            response["disk_used"] = used
            response["disk_total"] = _total
            response["disk_percentage"] = percentage

            if err != nil {
                fmt.Println(err)
                return
            }
            m, err := json.Marshal(response)
            if err != nil {
                fmt.Println(err)
                return
            }
            message = m
        }
        // write
        err = ws.WriteMessage(mt, message)
        if err != nil {
            break
        }
    }
}
