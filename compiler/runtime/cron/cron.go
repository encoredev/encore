package cron

import "time"

type JobConfig struct {
	Name     string
	Every    time.Duration
	Schedule string
	Endpoint interface{}
}

type Job struct {
	ID       string
	Name     string
	Doc      string
	Every    time.Duration
	Schedule string
	Endpoint interface{}
}

func NewJob(id string, jobConfig JobConfig) *Job {
	return &Job{
		ID:       id,
		Name:     jobConfig.Name,
		Doc:      jobConfig.Name,
		Every:    jobConfig.Every,
		Schedule: jobConfig.Schedule,
		Endpoint: jobConfig.Endpoint,
	}
}
