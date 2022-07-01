// Package cron provides support for cron jobs: recurring tasks that run on a schedule.
//
// For more information about Encore's cron job support, see https://encore.dev/docs/develop/cron-jobs.
package cron

// NewJob defines a new cron job. It is specially recognized by the Encore Parser
// and results in the Encore Platform provisioning the cron job on next deploy.
// Note that cron jobs do not automatically execute when running the application locally.
// To test the cron job implementation, test the target endpoint directly.
//
// The id argument is a unique identifier you give to each cron job. If you later
// refactor the code and move the cron job definition to another package, Encore uses
// this ID to keep track that it's the same cron job and not a different one.
//
// The fields provided in the JobConfig must be constant literals, as they are parsed
// directly by the Encore Platform and are not actually executed at runtime.
//
// To define a new cron job, call NewJob and assign it a package-level variable:
//
// 		import "encore.dev/cron"
//
// 		// Send a welcome email to everyone who signed up in the last two hours.
// 		var _ = cron.NewJob("welcome-email", cron.JobConfig{
// 			Title:    "Send welcome emails",
// 			Every:    2 * cron.Hour,
// 			Endpoint: SendWelcomeEmail,
// 		})
//
// 		// SendWelcomeEmail emails everyone who signed up recently.
// 		// It's idempotent: it only sends a welcome email to each person once.
// 		//encore:api private
// 		func SendWelcomeEmail(ctx context.Context) error {
// 			// ...
// 			return nil
// 		}
func NewJob(id string, jobConfig JobConfig) *Job {
	return &Job{
		ID:       id,
		Title:    jobConfig.Title,
		Every:    jobConfig.Every,
		Schedule: jobConfig.Schedule,
		Endpoint: jobConfig.Endpoint,
	}
}

// JobConfig represents the configuration of a single cron job.
//
// The fields provided in the JobConfig must be constant literals, as they are parsed
// directly by the Encore Platform and are not actually executed at runtime.
type JobConfig struct {
	// Title is the descriptive title of the cron job, typically a short sentence like "Send welcome emails".
	Title string

	// Endpoint is the Encore API endpoint that should be called when the cron job executes.
	// It must not take any parameters other than context.Conetxt; that is, its signature must be
	// either "func(context.Context) error" or "func(context.Context) (T, error)" for any type T.
	Endpoint interface{}

	// Every defines how often the cron job should execute.
	// You must either specify either Every or Schedule (but not both).
	//
	// In order to ensure a consistent delay between each run, the interval used must divide 24 hours evenly.
	// For example, 10 * cron.Minute and 6 * cron.Hour are both allowed (since 24 hours is evenly divisible by both),
	// whereas 7 * cron.Hour is not (since 24 is not evenly divisible by 7).
	//
	// The Encore compiler will catch this and give you a helpful error at compile-time if you try to use an invalid interval.
	Every Duration

	// Schedule defines when the cron job should execute, using a Cron Expression.
	// You must either specify either Every or Schedule (but not both).
	//
	// For more information on cron expressions, see https://en.wikipedia.org/wiki/Cron.
	Schedule string
}

// Job represents a created cron job. It can be inspected at runtime to determine information
// about the cron job.
type Job struct {
	ID       string
	Title    string
	Every    Duration
	Schedule string
	Endpoint interface{}
}

// Duration represents the duration between cron execution intervals, expressed in seconds.
// Specific durations can easily be achieved using constant expressions, such as:
//
//    cron.Hour + 30*cron.Minute // 90 minutes
//
type Duration int64

const (
	Minute Duration = 60
	Hour   Duration = 60 * Minute
)
