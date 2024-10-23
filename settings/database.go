package settings

type Database struct {
	Name string
}

var DatabaseSettings = &Database{
	Name: "database",
}

func (d *Database) GetName() string {
	return d.Name
}
