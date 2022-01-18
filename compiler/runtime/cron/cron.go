package cron

type Job struct {
	ID          string
	Name        string
	Description string
	Schedule    string
	Endpoint    interface{}
}

func New(j *Job) *Job {
	return j
}
