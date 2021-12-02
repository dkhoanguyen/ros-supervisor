package ros_supervisor

type MonitorRepository struct {
	Name        string `json:"name"`
	Branch      string `json:"branch"`
	Url         string `json:"url"`
	UpdateReady bool   `json:"update_ready"`
}

func MakeMonitorRepository(name, url string) *MonitorRepository {
	return &MonitorRepository{
		Name:        name,
		Url:         url,
		UpdateReady: false,
	}
}

func (repo *MonitorRepository) GetName() string {
	return repo.Name
}

func (repo *MonitorRepository) GetUrl() string {
	return repo.Url
}

func (repo *MonitorRepository) GetUpdateReadyStatus() bool {
	return repo.UpdateReady
}

func (repo *MonitorRepository) CheckUpdateReadyStatus() bool {
	return false
}

var _ Repository = (*MonitorRepository)(nil)
