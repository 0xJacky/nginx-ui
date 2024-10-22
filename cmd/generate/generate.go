package main

import (
	"flag"
	"fmt"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/uozi-tech/cosy/settings"
	"gorm.io/driver/sqlite"
	"gorm.io/gen"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"log"
	"path"
)

func main() {
	// specify the output directory (default: "./query")
	// ### if you want to query without context constrain, set mode gen.WithoutContext ###
	g := gen.NewGenerator(gen.Config{
		OutPath: "../../query",
		Mode:    gen.WithoutContext | gen.WithDefaultQuery,
		//if you want the nullable field generation property to be pointer type, set FieldNullable true
		FieldNullable: true,
		//if you want to assign field which has the default value in `Create` API, set FieldCoverable true, reference: https://gorm.io/docs/create.html#Default-Values
		FieldCoverable: true,
		// if you want to generate field with an unsigned integer type, set FieldSignable true
		/* FieldSignable: true,*/
		//if you want to generate index tags from the database, set FieldWithIndexTag true
		/* FieldWithIndexTag: true,*/
		//if you want to generate type tags from the database, set FieldWithTypeTag true
		/* FieldWithTypeTag: true,*/
		//if you need unit tests for query code, set WithUnitTest true
		/* WithUnitTest: true, */
	})

	// reuse the database connection in Project or create a connection here
	// if you want to use GenerateModel/GenerateModelAs, UseDB is necessary, or it will panic
	var confPath string
	flag.StringVar(&confPath, "config", "app.ini", "Specify the configuration file")
	flag.Parse()

	settings.Init(confPath)
	dbPath := path.Join(path.Dir(confPath), fmt.Sprintf("%s.db", settings.DataBaseSettings.Name))

	var err error
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger:                                   logger.Default.LogMode(logger.Info),
		PrepareStmt:                              true,
		DisableForeignKeyConstraintWhenMigrating: true,
	})

	if err != nil {
		log.Fatalln(err)
	}

	g.UseDB(db)

	// apply basic crud api on structs or table models which is specified by table name with function
	// GenerateModel/GenerateModelAs. And the generator will generate table models' code when calling Excute.
	g.ApplyBasic(model.GenerateAllModel()...)

	// apply diy interfaces on structs or table models
	g.ApplyInterface(func(method model.Method) {}, model.GenerateAllModel()...)

	// execute the action of code generation
	g.Execute()
}
