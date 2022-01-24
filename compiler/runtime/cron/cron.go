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
	panic("encore apps must be run using the encore command")
}
