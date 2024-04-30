package cosy

import (
    "github.com/0xJacky/Nginx-UI/internal/logger"
    "github.com/gin-gonic/gin"
    "github.com/go-playground/validator/v10"
    "gorm.io/gorm"
)

var validate *validator.Validate

func init() {
    validate = validator.New()
}

type Ctx[T any] struct {
    ctx                      *gin.Context
    rules                    gin.H
    Payload                  map[string]interface{}
    Model                    T
    OriginModel              T
    table                    string
    tableArgs                []interface{}
    abort                    bool
    nextHandler              *gin.HandlerFunc
    skipAssociationsOnCreate bool
    beforeDecodeHookFunc     []func(ctx *Ctx[T])
    beforeExecuteHookFunc    []func(ctx *Ctx[T])
    executedHookFunc         []func(ctx *Ctx[T])
    gormScopes               []func(tx *gorm.DB) *gorm.DB
    preloads                 []string
    scan                     func(tx *gorm.DB) any
    transformer              func(*T) any
    permanentlyDelete        bool
    SelectedFields           []string
    itemKey                  string
}

func Core[T any](c *gin.Context) *Ctx[T] {
    return &Ctx[T]{
        ctx:                      c,
        gormScopes:               make([]func(tx *gorm.DB) *gorm.DB, 0),
        beforeExecuteHookFunc:    make([]func(ctx *Ctx[T]), 0),
        beforeDecodeHookFunc:     make([]func(ctx *Ctx[T]), 0),
        itemKey:                  "`id`",
        skipAssociationsOnCreate: true,
    }
}

func (c *Ctx[T]) SetTable(table string, args ...interface{}) *Ctx[T] {
    c.table = table
    c.tableArgs = args
    return c
}

func (c *Ctx[T]) SetItemKey(key string) *Ctx[T] {
    c.itemKey = key
    return c
}

func (c *Ctx[T]) SetValidRules(rules gin.H) *Ctx[T] {
    c.rules = rules

    return c
}

func (c *Ctx[T]) SetPreloads(args ...string) *Ctx[T] {
    c.preloads = append(c.preloads, args...)
    return c
}

func (c *Ctx[T]) validate() (errs gin.H) {
    c.Payload = make(gin.H)

    _ = c.ctx.ShouldBindJSON(&c.Payload)

    errs = validate.ValidateMap(c.Payload, c.rules)

    if len(errs) > 0 {
        logger.Debug(errs)
        for k := range errs {
            errs[k] = c.rules[k]
        }
        return
    }
    // Make sure that the key in c.Payload is also the key of rules
    validated := make(map[string]interface{})

    for k, v := range c.Payload {
        if _, ok := c.rules[k]; ok {
            validated[k] = v
        }
    }

    c.Payload = validated

    return
}

func (c *Ctx[T]) SetScan(scan func(tx *gorm.DB) any) *Ctx[T] {
    c.scan = scan
    return c
}

func (c *Ctx[T]) SetTransformer(t func(m *T) any) *Ctx[T] {
    c.transformer = t
    return c
}

func (c *Ctx[T]) AbortWithError(err error) {
    c.abort = true
    errHandler(c.ctx, err)
}

func (c *Ctx[T]) Abort() {
    c.abort = true
}
