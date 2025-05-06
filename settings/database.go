package settings

type Database struct {
	Name string `json:"name"`
}

var DatabaseSettings = &Database{
	Name: "database",
}

func (d *Database) GetName() string {
	if d.Name == "" {
		d.Name = "database"
	}
	return d.Name
}
