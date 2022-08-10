package db

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Project struct {
	gorm.Model
	ProjectID string
	Name      string `gorm:"primaryKey"`
	Version   string `gorm:"primaryKey"`
}

func AddProject(
	name string,
	version string,
	db *Database) error {

	id := uuid.New()
	result := db.Db.Where(&Project{
		Name: name,
	}).FirstOrCreate(&Project{
		Name:      name,
		ProjectID: id.String(),
		Version:   version,
	})

	return result.Error
}

func GetProjectByNameAndVersion(
	name string,
	version string,
	db *Database) (Project, error) {

	var project Project

	result := db.Db.Where(&Project{
		Name:    name,
		Version: version,
	}).First(&project)

	return project, result.Error
}

func DeleteProjectByNameAndVersion(
	name string,
	version string,
	db *Database) error {
	result := db.Db.Delete(&Project{
		Name:    name,
		Version: version,
	})

	return result.Error
}
