package cron

type Duration int64

const (
	Minute Duration = 60
	Hour   Duration = 60 * Minute
)

type JobConfig struct {
	Title    string
	Every    Duration
	Schedule string
	Endpoint interface{}
}

type Job struct {
	ID       string
	Title    string
	Every    Duration
	Schedule string
	Endpoint interface{}
}

func NewJob(id string, jobConfig JobConfig) *Job {
	return &Job{
		ID:       id,
		Title:    jobConfig.Title,
		Every:    jobConfig.Every,
		Schedule: jobConfig.Schedule,
		Endpoint: jobConfig.Endpoint,
	}
}
