package cron

type Duration int64

const (
	Minute Duration = 60
	Hour   Duration = 60 * Minute
)

type JobConfig struct {
	Name     string
	Every    Duration
	Schedule string
	Endpoint interface{}
}

type Job struct {
	ID       string
	Name     string
	Every    Duration
	Schedule string
	Endpoint interface{}
}

func NewJob(id string, jobConfig JobConfig) *Job {
	return &Job{
		ID:       id,
		Name:     jobConfig.Name,
		Every:    jobConfig.Every,
		Schedule: jobConfig.Schedule,
		Endpoint: jobConfig.Endpoint,
	}
}
