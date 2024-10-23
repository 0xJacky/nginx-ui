package settings

type Database struct {
	Name string `json:"name"`
}

var DatabaseSettings = &Database{
	Name: "database",
}

func (d *Database) GetName() string {
	return d.Name
}
