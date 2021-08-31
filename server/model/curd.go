package model

type Curd struct {
    Model interface{}
}

func NewCurd(Model interface{}) *Curd {
    return &Curd{Model: Model}
}

func (c *Curd) GetList(dest interface{}) (err error) {
    err = db.Model(c.Model).Scan(dest).Error
    return
}

func (c *Curd) First(dest interface{}, conds ...interface{}) (err error) {
    err = db.Model(c.Model).First(dest, conds).Error
    return
}

func (c *Curd) Add(value interface{}) (err error) {
    err = db.Model(c.Model).Create(value).Error
    if err != nil {
        return err
    }
    err = db.Find(value).Error
    return
}

func (c *Curd) Edit(orig interface{}, new interface{}) (err error) {
    err = db.Model(orig).Updates(new).Error
    return
}

func (c *Curd) Delete(value interface{}, conds ...interface{}) (err error) {
    err = db.Model(c.Model).Delete(value, conds).Error
    return
}
