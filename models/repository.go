package models

type Repository interface {
	GetName() string
	GetUrl() string
	GetUpdateReadyStatus() bool
}
