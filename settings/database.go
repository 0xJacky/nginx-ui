package settings

type Database struct {
	Name string
}

var DatabaseSettings = &Database{}

func (d *Database) GetName() string {
	return d.Name
}
