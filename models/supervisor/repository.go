package ros_supervisor

type Repository interface {
	GetName() string
	GetUrl() string
	GetUpdateReadyStatus() bool
}
