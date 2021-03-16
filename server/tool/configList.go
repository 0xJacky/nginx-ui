package tool

import (
    "github.com/gin-gonic/gin"
    "sort"
    "time"
)

type MapsSort struct {
    Key     string
    Type    string
    Order   string
    MapList []gin.H
}

func boolToInt(b bool) int {
    if b {
        return 1
    }
    return 0
}

func (m MapsSort) Len() int {
    return len(m.MapList)
}

func (m MapsSort) Less(i, j int) bool {
    flag := false

    if m.Type == "int" {
        flag = m.MapList[i][m.Key].(int) > m.MapList[j][m.Key].(int)
    } else if m.Type == "bool" {
        flag = boolToInt(m.MapList[i][m.Key].(bool)) > boolToInt(m.MapList[j][m.Key].(bool))
    } else if m.Type == "bool" {
        flag = m.MapList[i][m.Key].(string) > m.MapList[j][m.Key].(string)
    } else if m.Type == "time" {
        flag = m.MapList[i][m.Key].(time.Time).After(m.MapList[j][m.Key].(time.Time))
    }

    if m.Order == "asc" {
        flag = !flag
    }

    return flag
}

func (m MapsSort) Swap(i, j int) {
    m.MapList[i], m.MapList[j] = m.MapList[j], m.MapList[i]
}

func Sort(key string, order string, Type string, maps []gin.H) []gin.H {
    mapsSort := MapsSort{
        Key: key,
        MapList: maps,
        Type: Type,
        Order: order,
    }

    sort.Sort(mapsSort)

    return mapsSort.MapList
}
