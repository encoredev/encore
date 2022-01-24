package cron

type JobConfig struct {
	Name     string
	Schedule string
	Endpoint interface{}
}

type Job struct {
	ID       string
	Name     string
	Doc      string
	Schedule string
	Endpoint interface{}
}

func NewJob(id string, jobConfig JobConfig) *Job {
	return &Job{
		ID:       id,
		Name:     jobConfig.Name,
		Doc:      jobConfig.Name,
		Schedule: jobConfig.Schedule,
		Endpoint: jobConfig.Endpoint,
	}
}
