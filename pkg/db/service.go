package db

import (
	"gorm.io/gorm"
)

type Service struct {
	gorm.Model
	Version string `gorm:"primaryKey"`
	Name    string `gorm:"primaryKey"`

	ProjectID   string
	ImageID     string
	ContainerID string
	NetworkID   string
	VolumeID    string

	RawData []byte
}

func AddService(
	name string,
	version string,
	rawData []byte,
	db *Database) error {

	result := db.Db.Where(&Service{
		Name: name,
	}).FirstOrCreate(&Service{
		Name:    name,
		RawData: rawData,
	})

	return result.Error
}

// In the future, add project id
func GetServiceByNameAndVersion(
	name string,
	version string,
	db *Database) (Service, error) {

	var service Service

	result := db.Db.Where(&Service{
		Name:    name,
		Version: version,
	}).First(&service)

	return service, result.Error
}

func DeleteServiceByNameAndVersion(
	name string,
	version string,
	db *Database) error {
	result := db.Db.Delete(&Service{
		Name:    name,
		Version: version,
	})

	return result.Error
}
