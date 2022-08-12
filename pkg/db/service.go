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

	RawDockerData []byte
	RawConfigData []byte

	ProcessedRawData []byte
}

func AddService(
	name string,
	version string,
	rawData []byte,
	db *Database) error {

	result := db.Db.Where(&Service{
		Name: name,
	}).FirstOrCreate(&Service{
		Name:          name,
		Version:       version,
		RawDockerData: rawData,
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

func UpdateDockerConfig(
	name string,
	config []byte,
	db *Database) error {
	service := Service{}
	result := db.Db.Where(&Service{
		Name: name,
	}).First(&service)
	if result.Error != nil {
		return result.Error
	}
	service.RawDockerData = config
	db.Db.Save(&service)
	return nil
}

func UpdateSupervisorConfig(
	name string,
	config []byte,
	db *Database) error {
	service := Service{}
	result := db.Db.Where(&Service{
		Name: name,
	}).First(&service)
	if result.Error != nil {
		return result.Error
	}
	service.RawConfigData = config
	db.Db.Save(&service)
	return nil
}

func UpdateServiceRawData(
	name string,
	rawData []byte,
	db *Database) error {
	service := Service{}
	result := db.Db.Where(&Service{
		Name: name,
	}).First(&service)
	if result.Error != nil {
		return result.Error
	}
	service.ProcessedRawData = rawData
	db.Db.Save(&service)
	return nil
}

func UpdateServiceNetworkID(
	name string,
	networkId string,
	db *Database) error {
	service := Service{}
	result := db.Db.Where(&Service{
		Name: name,
	}).First(&service)
	if result.Error != nil {
		return result.Error
	}
	service.NetworkID = networkId
	db.Db.Save(&service)
	return nil
}

func UpdateServiceVolumeID(
	name string,
	volumeId string,
	db *Database) error {
	service := Service{}
	result := db.Db.Where(&Service{
		Name: name,
	}).First(&service)
	if result.Error != nil {
		return result.Error
	}
	service.VolumeID = volumeId
	db.Db.Save(&service)
	return nil
}

func UpdateServiceImageID(
	name string,
	imageId string,
	db *Database) error {
	service := Service{}
	result := db.Db.Where(&Service{
		Name: name,
	}).First(&service)
	if result.Error != nil {
		return result.Error
	}
	service.ImageID = imageId
	db.Db.Save(&service)
	return nil
}

func UpdateServiceContainerID(
	name string,
	containerId string,
	db *Database) error {
	service := Service{}
	result := db.Db.Where(&Service{
		Name: name,
	}).First(&service)
	if result.Error != nil {
		return result.Error
	}
	service.ContainerID = containerId
	db.Db.Save(&service)
	return nil
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
