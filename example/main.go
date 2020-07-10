package main

import (
	"fmt"

	_ "github.com/denisenkom/go-mssqldb"
	"github.com/gogf/gf/database/gdb"
	"github.com/sanrentai/automigrate"
	_ "github.com/sanrentai/automigrate/dialects/mssql"
)

type MyTest struct {
	automigrate.Model
	Name string
}

func (t MyTest) TableName() string {
	return "my_test"
}

func main() {
	gdb.SetConfig(gdb.Config{
		"default": gdb.ConfigGroup{
			gdb.ConfigNode{
				Host: "localhost",
				Port: "1433",
				User: "sa",
				Pass: "8548",
				Name: "mydemo",
				Type: "mssql",
			},
		},
	})

	db, err := gdb.Instance()
	if err != nil {
		panic(err)
	}

	rs, err := db.GetAll("select * from foo")
	if err != nil {
		panic(err)
	}
	fmt.Println(rs)

	adb := automigrate.NewDB("mssql", db)

	adb.AutoMigrate(&MyTest{})
}
