package db

import (
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type Database struct {
	Db   *gorm.DB
	Path string
}

func MakeDatabase(path string) Database {
	output := Database{
		Path: path,
	}

	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {

	}
	db.AutoMigrate(&Service{})
	output.Db = db
	return output
}
