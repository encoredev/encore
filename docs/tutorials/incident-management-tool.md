---
title: Building an Incident Management Tool
subtitle: Set up your own PagerDuty from zero-to-production in just 30 minutes
social_card: /assets/docs/incident-og-image.png
---

In this tutorial, we're going to walk through together how to build our very own Incident Management Tool like [Incident.io](https://incident.io) or [PagerDuty](https://pagerduty.com). We can then have our own on call schedule that can be rotated between many users, and have incidents come and be assigned according to the schedule!

![Slack Incident Management Tool](/assets/docs/incident-slack-example.png "Incident Management Tool")

In about 30 minutes, your application will be able to support:

- Creating users, as well as schedules for when users will be on call
- Creating incidents, and reminders for unacknowledged incidents on Slack every 10 minutes
- Auto-assign incidents which are unassigned (when the next user is on call)

_ Sounds good? Let's dig in! _

Or if you'd rather watch a video of this tutorial, you can do that below.

<iframe width="360" height="202" src="https://www.youtube.com/embed/BR_ys_qR2kI?controls=0" title="Building an Incident Management Tool Video Tutorial" frameborder="0" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture" allowfullscreen></iframe>


<Callout type="info">

To make it easier to follow along, we've laid out a trail of croissants to guide your way.
Whenever you see a 🥐 it means there's something for you to do.

</Callout>

## Create your Encore application

🥐 Create a new Encore application by running `encore app create` and select `Empty app` as the template. We can name it `oncall-tutorial` for now. Your newly created application will also be registered on <https://app.encore.dev> for when you deploy your new app later!

🥐 Run `cd oncall-tutorial` in your Terminal otherwise Encore won't be able to understand which app you're referring to when you run the `encore` CLI!

## Integrating with Slack

🥐 Follow [this guide to create your own Incoming Webhook](https://api.slack.com/messaging/webhooks) for your Slack workspace. Incoming webhooks cannot read messages, and can only post to a specific channel of your choice.

<img src="/assets/docs/incident-slack-app-creation.png" alt="Creating a Slack app" width="400px" />

🥐 Once you have your Webhook URL which starts with `https://hooks.slack.com/services/...` then copy and paste that and run the following commands to save these as secrets. We recommend having a different webhook/channel for development and production.

```bash
encore secret set --dev SlackWebhookURL
encore secret set --prod SlackWebhookURL
```

🥐 Next, let's create our `slack` service that contains the logic for calling the Webhook URL in order to post notifications to our Slack. To do this we need to implement our code in `slack/slack.go`:

```go
package slack

import (
	"bytes"
	"context"
	"encoding/json"
	"encore.dev/beta/errs"
	"io"
	"net/http"
)

type NotifyParams struct {
	Text string `json:"text"`
}

//encore:api private
func Notify(ctx context.Context, p *NotifyParams) error {
	eb := errs.B()
	reqBody, err := json.Marshal(p)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", secrets.SlackWebhookURL, bytes.NewReader(reqBody))
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return eb.Code(errs.Unavailable).Msgf("notify slack: %s: %s", resp.Status, body).Err()
	}
	return nil
}

var secrets struct {
	SlackWebhookURL string
}
```

<Callout type="info">

The `slack` service can be reused across any of your Encore apps. All you need is the `slack/slack.go` code and the `SlackWebhookURL` secret to be defined. Then you can call the following method signature anywhere in your app:

```go
slack.Notify(context, &slack.NotifyParams{ Text: "Send a Slack notification" })
```

</Callout>

## Creating users

With an Incident Management Tool (or usually any tool, for that matter) we need a service for users.
This will allow us to figure out who we should assign incoming incidents to!

To get started, we need to create a `users` service with the following resources:

| # | Type | Description / Filename |
| - | - | - |
| #1 | SQL Migration | Our PostgreSQL schema for scheduling data <br/> `users/migrations/1_create_users.up.sql` |
| #2 | HTTP Endpoint <br/> `POST /users` | Create a new User <br/> `users/users.go` |
| #3 | HTTP Endpoint <br/> `GET /users/:id` | Get an existing User <br/> `users/users.go` |

With #1, let's design our database schema for a User in our system. For now let's store a first and last name as well as a Slack handle in case we need to notify them about any incidents which may have been assigned to them or acknowledged by them.

🥐 Let's create our migration file in `users/migrations/1_create_users.up.sql`: 

```sql
CREATE TABLE users ( 
    id           BIGSERIAL PRIMARY KEY,
    first_name   VARCHAR(255) NOT NULL,
    last_name    VARCHAR(255) NOT NULL,
    slack_handle VARCHAR(255) NOT NULL
);
```

🥐 Then, we need to write our code to implement the HTTP endpoints listed in #2 (for creating a user) and #3 (for listing a user) belonging in `users/users.go`. Let's split them out into three sections: our structs (i.e. data models) and methods.

```go
package users

import (
	"context"
	"encore.dev/storage/sqldb"
)

// This is a Go struct representing our PostgreSQL schema for `users`
type User struct {
	Id          int32
	FirstName   string
	LastName    string
	SlackHandle string
}

//encore:api public method=POST path=/users
func Create(ctx context.Context, params CreateParams) (*User, error) {
	user := User{}
	err := sqldb.QueryRow(ctx, `
		INSERT INTO users (first_name, last_name, slack_handle)
		VALUES ($1, $2, $3)
		RETURNING id, first_name, last_name, slack_handle
	`, params.FirstName, params.LastName, params.SlackHandle).Scan(&user.Id, &user.FirstName, &user.LastName, &user.SlackHandle)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// This is what JSON params our POST /users endpoint will accept
type CreateParams struct {
	FirstName   string
	LastName    string
	SlackHandle string
}

//encore:api public method=GET path=/users/:id
func Get(ctx context.Context, id int32) (*User, error) {
	user := User{}
	err := sqldb.QueryRow(ctx, `
	  SELECT id, first_name, last_name, slack_handle
		FROM users
		WHERE id = $1
	`, id).Scan(&user.Id, &user.FirstName, &user.LastName, &user.SlackHandle)
	if err != nil {
		return nil, err
	}
	return &user, nil
}
```

🥐 Next, type `encore run` in your Terminal and in a separate window run the command under **cURL Request** (feel free to edit the values!) to create our first user:

```bash
curl -d '{
  "FirstName":"Katy",
  "LastName":"Smith",
  "SlackHandle":"katy"
}' http://localhost:4000/users

# Example JSON response
# {
#   "Id":1,
#   "FirstName":"Katy",
#   "LastName":"Smith",
#   "SlackHandle":"katy"
# }
```

Fantastic, we now have a user system in our app! Next we need a list of start and end times of each scheduled rotation so we know who to assign incoming incidents to (as well as notify them on Slack!)

## Adding scheduling

A good incident management tool should be able to spread the workload of diagnosing and fixing incidents across multiple users in a team. Being able to know who the correct person to assign an incident to is very important; our incidents might not get resolved quickly otherwise!

In order to achieve this, let's create a new service called `schedules`:

| # | Type | Description / Filename |
| - | - | - |
| #1 | SQL Migration | Our PostgreSQL schema for user data <br/> `schedules/migrations/1_create_schedules.up.sql` |
| #2 | HTTP Endpoint <br/> `GET /schedules` | Get list of schedules between time range <br/> `schedules/schedules.go` |
| #3 | HTTP Endpoint <br/> `POST /users/:id/schedules` | Create a new Schedule <br/> `schedules/schedules.go` |
| #4 | HTTP Endpoint <br/> `GET /scheduled/:timestamp` | Get Schedule at specific time <br/> `schedules/schedules.go` |


For the SQL migration in #1, we need to create both a table and an index. For every rotation let's need a new entry containing the user who it is for as well as the start and end times of the scheduled rotation.

🥐 Let's create our migration file in `schedules/migrations/1_create_schedules.up.sql`:

```sql
CREATE TABLE schedules
(
    id         BIGSERIAL PRIMARY KEY,
    user_id    INTEGER   NOT NULL,
    start_time TIMESTAMP NOT NULL,
    end_time   TIMESTAMP NOT NULL
);

CREATE INDEX schedules_range_index ON schedules (start_time, end_time);
```


<Callout type="info">

Table indexes are used to optimize lookups without having to search every row in the table. In this case, looking up rows against both `start_time` and `end_time` will be faster _with the index_ as the dataset grows. [Learn more about PostgreSQL indexes here](https://www.tutorialspoint.com/postgresql/postgresql_indexes.htm).

</Callout>

🥐 Next, let's implement the HTTP endpoints for #2 (listing schedules), #3 (creating a schedule) and #4 (getting the schedule/user at a specific time) in `schedules/schedules.go`:

```go
package schedules

import (
	"context"
	"errors"
	"time"

	"encore.app/users"
	"encore.dev/beta/errs"
	"encore.dev/storage/sqldb"
)

// This struct holds multiple Schedule structs
type Schedules struct {
	Items []Schedule
}

// This is a Go struct representing our PostgreSQL schema for `schedules`
type Schedule struct {
	Id   int32
	User users.User
	Time TimeRange
}

// As we use time ranges in our schedule, we created a generic TimeRange struct
type TimeRange struct {
	Start time.Time
	End   time.Time
}

//encore:api public method=POST path=/users/:userId/schedules
func Create(ctx context.Context, userId int32, timeRange TimeRange) (*Schedule, error) {
	eb := errs.B().Meta("userId", userId, "timeRange", timeRange)
	// check for existing overlapping schedules
	if schedule, err := ScheduledAt(ctx, timeRange.Start.String()); schedule != nil && err == nil {
		return nil, eb.Code(errs.InvalidArgument).Cause(err).Msg("schedule already exists within this start timestamp").Err()
	}
	if schedule, err := ScheduledAt(ctx, timeRange.End.String()); schedule != nil && err == nil {
		return nil, eb.Code(errs.InvalidArgument).Cause(err).Msg("schedule already exists within this end timestamp").Err()
	}

	// check user exists
	user, err := users.Get(ctx, userId)
	if err != nil {
		return nil, eb.Code(errs.Unavailable).Cause(err).Msg("failed to get user").Err()
	}

	schedule := Schedule{User: *user, Time: TimeRange{}}
	err = sqldb.QueryRow(
		ctx,
		`INSERT INTO schedules (user_id, start_time, end_time) VALUES ($1, $2, $3) RETURNING id, start_time, end_time`,
		userId, timeRange.Start, timeRange.End,
	).Scan(&schedule.Id, &schedule.Time.Start, &schedule.Time.End)
	if err != nil {
		return nil, eb.Code(errs.Unavailable).Cause(err).Msg("failed to insert schedule").Err()
	}

	return &schedule, nil
}

//encore:api public method=GET path=/scheduled/:timestamp
func ScheduledAt(ctx context.Context, timestamp string) (*Schedule, error) {
	eb := errs.B().Meta("timestamp", timestamp)
	parsedtime, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		return nil, eb.Code(errs.InvalidArgument).Msg("timestamp is not in a valid format").Err()
	}

	return Scheduled(ctx, parsedtime)
}

func Scheduled(ctx context.Context, timestamp time.Time) (*Schedule, error) {
	eb := errs.B().Meta("timestamp", timestamp)
	schedule, err := RowToSchedule(ctx, sqldb.QueryRow(ctx, `
		SELECT id, user_id, start_time, end_time
		FROM schedules
		WHERE start_time <= $1
		  AND end_time >= $1
	`, timestamp.UTC()))
	if errors.Is(err, sqldb.ErrNoRows) {
		return nil, eb.Code(errs.NotFound).Msg("no schedule found").Err()
	}
	if err != nil {
		return nil, err
	}
	return schedule, nil
}

//encore:api public method=GET path=/schedules
func ListByTimeRange(ctx context.Context, timeRange TimeRange) (*Schedules, error) {
	rows, err := sqldb.Query(ctx, `
		SELECT id, user_id, start_time, end_time
		FROM schedules
		WHERE start_time >= $1
		AND end_time <= $2
		ORDER BY start_time ASC
	`, timeRange.Start, timeRange.End)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var schedules []Schedule
	for rows.Next() {
		schedule, err := RowToSchedule(ctx, rows)
		if err != nil {
			return nil, err
		}
		schedules = append(schedules, *schedule)
	}

	return &Schedules{Items: schedules}, nil
}

//encore:api public method=DELETE path=/schedules
func DeleteByTimeRange(ctx context.Context, timeRange TimeRange) (*Schedules, error) {
	schedules, err := ListByTimeRange(ctx, timeRange)
	if err != nil {
		return nil, err
	}
	_, err = sqldb.Exec(ctx, `DELETE FROM schedules WHERE start_time >= $1 AND end_time <= $2`, timeRange.Start, timeRange.End)
	if err != nil {
		return nil, err
	}

	return schedules, err
}

// Helper function to convert a Row object to to Schedule
func RowToSchedule(ctx context.Context, row interface {
	Scan(dest ...interface{}) error
}) (*Schedule, error) {
	var schedule = &Schedule{Time: TimeRange{}}
	var userId int32

	err := row.Scan(&schedule.Id, &userId, &schedule.Time.Start, &schedule.Time.End)
	if err != nil {
		return nil, err
	}

	user, err := users.Get(ctx, userId)
	if err != nil {
		return nil, err
	}

	schedule.User = *user
	return schedule, nil
}
```

🥐 Next, type `encore run` in your Terminal and in a separate window run the command under **cURL Request** (also feel free to edit the values!) to create our first schedule against the user we created earlier:

```bash
curl -d '{
  "Start":"2023-11-28T10:00:00Z",
  "End":"2023-11-30T10:00:00Z"
}' "http://localhost:4000/users/1/schedules"

# Example JSON response
# {
#   "Id":1,
#   "User":{
#     "Id":1,
#     "FirstName":"Katy",
#     "LastName":"Smith",
#     "SlackHandle":"katy"
#   },
#   "Time":{
#     "Start":"2023-11-28T10:00:00Z",
#     "End":"2023-11-30T10:00:00Z"
#   }
# }
```

## Creating incidents: this is fine

So we have users, and we know who is available to be notified (or if nobody should be notified) at any given time with the introduction of the `schedules` service. The only thing we're missing is the ability to report, assign and acknowledge incidents!

The flow we're going to implement is: an incoming incident will arrive, let's either unassign or auto-assign it based on the `schedules` service, and incidents have to be acknowledged. If they are not acknowledged, they will continue to be notified on Slack every 10 minutes until it has.

To start with, we need to create a new `incidents` service with the following resources:


| # | Type | Description / Filename |
| - | - | - |
| #1 | SQL Migration | Our PostgreSQL schema for storing incidents <br/> `incidents/migrations/1_create_incidents.up.sql` |
| #2 | HTTP Endpoint <br/> `GET /incidents` | Get list of all unacknowledged incidents <br/> `incidents/incidents.go` |
| #3 | HTTP Endpoint <br/> `PUT /incidents/:id/acknowledge` | Acknowledge an incident <br/> `incidents/incidents.go` |
| #4 | HTTP Endpoint <br/> `GET /scheduled/:timestamp` | Get  <br/> `incidents/incidents.go` |

For the SQL migration in #1, we need to create the table for our incidents. We need to have a one-to-many relationship between an user and an incident. That is, an incident can only be assigned to a single user but a single user can be assigned to many incidents.

🥐 Let's create our migration file in `incidents/migrations/1_create_incidents.up.sql`:

```sql
CREATE TABLE incidents
(
    id               BIGSERIAL PRIMARY KEY,
    assigned_user_id INTEGER,
    body             TEXT      NOT NULL,
    created_at       TIMESTAMP NOT NULL DEFAULT NOW(),
    acknowledged_at  TIMESTAMP
);
```

🥐 Next, our code belonging in `incidents/incidents.go` for being able to support incidents is below:

```go
package incidents

import (
	"context"
	"encore.app/schedules"
	"encore.app/slack"
	"encore.app/users"
	"encore.dev/beta/errs"
	"encore.dev/storage/sqldb"
	"fmt"
	"time"
)

// This struct holds multiple Incidents structs
type Incidents struct {
	Items []Incident
}

// This is a Go struct representing our PostgreSQL schema for `incidents`
type Incident struct {
	Id             int32
	Body           string
	CreatedAt      time.Time
	Acknowledged   bool
	AcknowledgedAt *time.Time
	Assignee       *users.User
}

//encore:api public method=GET path=/incidents
func List(ctx context.Context) (*Incidents, error) {
	rows, err := sqldb.Query(ctx, `
		SELECT id, assigned_user_id, body, created_at, acknowledged_at
		FROM incidents
		WHERE acknowledged_at IS NULL
	`)
	if err != nil {
		return nil, err
	}
	return RowsToIncidents(ctx, rows)
}

//encore:api public method=PUT path=/incidents/:id/acknowledge
func Acknowledge(ctx context.Context, id int32) (*Incident, error) {
	eb := errs.B().Meta("incidentId", id)
	rows, err := sqldb.Query(ctx, `
		UPDATE incidents
		SET acknowledged_at = NOW()
		WHERE acknowledged_at IS NULL
		  AND id = $1
		RETURNING id, assigned_user_id, body, created_at, acknowledged_at
	`, id)
	if err != nil {
		return nil, err
	}

	incidents, err := RowsToIncidents(ctx, rows)
	if err != nil {
		return nil, err
	}
	if incidents.Items == nil {
		return nil, eb.Code(errs.NotFound).Msg("no incident found").Err()
	}

	incident := &incidents.Items[0]
	_ = slack.Notify(ctx, &slack.NotifyParams{
		Text: fmt.Sprintf("Incident #%d assigned to %s %s <@%s> has been acknowledged:\n%s", incident.Id, incident.Assignee.FirstName, incident.Assignee.LastName, incident.Assignee.SlackHandle, incident.Body),
	})

	return incident, err
}

//encore:api public method=POST path=/incidents
func Create(ctx context.Context, params *CreateParams) (*Incident, error) {
	// check who is on-call
	schedule, err := schedules.Scheduled(ctx, time.Now())

	incident := Incident{}
	if schedule != nil {
		incident.Assignee = &schedule.User
	}

	var row *sqldb.Row
	if schedule != nil {
		// Someone is on-call
		row = sqldb.QueryRow(ctx, `
			INSERT INTO incidents (assigned_user_id, body)
			VALUES ($1, $2)
			RETURNING id, body, created_at
		`, &schedule.User.Id, params.Body)
	} else {
		// Nobody is on-call
		row = sqldb.QueryRow(ctx, `
			INSERT INTO incidents (body)
			VALUES ($1)
			RETURNING id, body, created_at
		`, params.Body)
	}

	if err = row.Scan(&incident.Id, &incident.Body, &incident.CreatedAt); err != nil {
		return nil, err
	}

	var text string
	if incident.Assignee != nil {
		text = fmt.Sprintf("Incident #%d created and assigned to %s %s <@%s>\n%s", incident.Id, incident.Assignee.FirstName, incident.Assignee.LastName, incident.Assignee.SlackHandle, incident.Body)
	} else {
		text = fmt.Sprintf("Incident #%d created and unassigned\n%s", incident.Id, incident.Body)
	}
	_ = slack.Notify(ctx, &slack.NotifyParams{Text: text})

	return &incident, nil
}

type CreateParams struct {
	Body string
}

// Helper to take a sqldb.Rows instance and convert it into a list of Incidents
func RowsToIncidents(ctx context.Context, rows *sqldb.Rows) (*Incidents, error) {
	eb := errs.B()

	defer rows.Close()

	var incidents []Incident
	for rows.Next() {
		var incident = Incident{}
		var assignedUserId *int32
		if err := rows.Scan(&incident.Id, &assignedUserId, &incident.Body, &incident.CreatedAt, &incident.AcknowledgedAt); err != nil {
			return nil, eb.Code(errs.Unknown).Msgf("could not scan: %v", err).Err()
		}
		if assignedUserId != nil {
			user, err := users.Get(ctx, *assignedUserId)
			if err != nil {
				return nil, eb.Code(errs.NotFound).Msgf("could not retrieve user for incident %v", assignedUserId).Err()
			}
			incident.Assignee = user
		}
		incident.Acknowledged = incident.AcknowledgedAt != nil
		incidents = append(incidents, incident)
	}

	return &Incidents{Items: incidents}, nil
}
```

Fantastic! We have an _almost_ working application. The main two things we're missing are:

1. For unacknowledged incidents, we need to post a reminder on Slack every 10 minutes until they have been acknolwedged.
2. Whenever a user is currently on call, we should assign all previously unassigned incidents to them. 

🥐 To achieve this, we'll need to create two [Cron Jobs](http://localhost:3000/docs/develop/cron-jobs) which thankfully Encore makes incredibly simple. So let's go ahead and create the first one for reminding us every 10 minutes of incidents we haven't acknowledged. Go ahead and add the code below to our `incidents/incidents.go` file:

```go
// Track unacknowledged incidents
var _ = cron.NewJob("unacknowledged-incidents-reminder", cron.JobConfig{
	Title:    "Notify on Slack about incidents which are not acknowledged",
	Every:    10 * cron.Minute,
	Endpoint: RemindUnacknowledgedIncidents,
})

//encore:api private
func RemindUnacknowledgedIncidents(ctx context.Context) error {
	incidents, err := List(ctx) // we never query for acknowledged incidents
	if err != nil {
		return err
	}
	if incidents == nil {
		return nil
	}

	var items = []string{"These incidents have not been acknowledged yet. Please acknowledge them otherwise you will be reminded every 10 minutes:"}
	for _, incident := range incidents.Items {
		var assignee string
		if incident.Assignee != nil {
			assignee = fmt.Sprintf("%s %s (<@%s>)", incident.Assignee.FirstName, incident.Assignee.LastName, incident.Assignee.SlackHandle)
		} else {
			assignee = "Unassigned"
		}

		items = append(items, fmt.Sprintf("[%s] [#%d] %s", assignee, incident.Id, incident.Body))
	}

	if len(incidents.Items) > 0 {
		_ = slack.Notify(ctx, &slack.NotifyParams{Text: strings.Join(items, "\n")})
	}

	return nil
}
```

And for our second cronjob, when someone goes on call we need to automatically assign the previously unassigned incidents to them. We don't have a HTTP endpoint for assigning incidents so we need to implement a `PUT /incidents/:id/assign` endpoint. 

🥐 So let's also add that endpoint as well as the cronjob code to our `incidents/incidents.go` file:

```go
//encore:api public method=PUT path=/incidents/:id/assign
func Assign(ctx context.Context, id int32, params *AssignParams) (*Incident, error) {
	eb := errs.B().Meta("params", params)
	rows, err := sqldb.Query(ctx, `
		UPDATE incidents
		SET assigned_user_id = $1
		WHERE acknowledged_at IS NULL
		  AND id = $2
		RETURNING id, assigned_user_id, body, created_at, acknowledged_at
	`, params.UserId, id)
	if err != nil {
		return nil, err
	}

	incidents, err := RowsToIncidents(ctx, rows)
	if err != nil {
		return nil, err
	}
	if incidents.Items == nil {
		return nil, eb.Code(errs.NotFound).Msg("no incident found").Err()
	}

	incident := &incidents.Items[0]
	_ = slack.Notify(ctx, &slack.NotifyParams{
		Text: fmt.Sprintf("Incident #%d is re-assigned to %s %s <@%s>\n%s", incident.Id, incident.Assignee.FirstName, incident.Assignee.LastName, incident.Assignee.SlackHandle, incident.Body),
	})

	return incident, err
}

type AssignParams struct {
	UserId int32
}

var _ = cron.NewJob("assign-unassigned-incidents", cron.JobConfig{
	Title:    "Assign unassigned incidents to user on-call",
	Every:    1 * cron.Minute,
	Endpoint: AssignUnassignedIncidents,
})

//encore:api private
func AssignUnassignedIncidents(ctx context.Context) error {
	// if this fails, we don't have anyone on call so let's skip this
	schedule, err := schedules.Scheduled(ctx, time.Now())
	if err != nil {
		return err
	}

	incidents, err := List(ctx) // we never query for acknowledged incidents
	if err != nil {
		return err
	}

	for _, incident := range incidents.Items {
		if incident.Assignee != nil {
			continue // this incident has already been assigned
		}

		_, err := Assign(ctx, incident.Id, &AssignParams{UserId: schedule.User.Id})
		if err == nil {
			rlog.Info("OK assigned unassigned incident", "incident", incident, "user", schedule.User)
		} else {
			rlog.Error("FAIL to assign unassigned incident", "incident", incident, "user", schedule.User, "err", err)
			return err
		}
	}

	return nil
}
```

🥐 Next, call `encore run` in your Terminal and in a separate window run the command under **cURL Request** (also feel free to edit the values!) to trigger our first incident. Most likely we won't have an assigned user unless you have scheduled a time that overlaps with right now in the last cURL request for creating a schedule:

```bash
curl -d '{
  "Body":"An unexpected error happened on example-website.com on line 38. It needs addressing now!"
}' http://localhost:4000/incidents

# Example JSON response
# {
#   "Id":1,
#	  "Body":"An unexpected error happened on example-website.com on line 38. It needs addressing now!",
#	  "CreatedAt":"2022-09-28T15:09:00Z",
#	  "Acknowledged":false,
#	  "AcknowledgedAt":null,
#   "Assignee":null
# }
```

## Finished

Congratulations! Our application looks ready for others to try - we have our `users`, `schedules` `incidents` and `slack` services along with 3 database tables and 2 cronjobs. Even better that all of the deployment and maintenance is taken care by Encore!

🥐 To try out your application, type `encore run` in your Terminal and run the following cURL commands:

```bash
# Step 1: Create a User and copy the User ID to your clipboard
curl -d '{
  "FirstName":"Katy",
  "LastName":"Smith",
  "SlackHandle":"katy"
}' http://localhost:4000/users

# Step 2: Create a schedule for the user we just created
curl -d '{
  "Start":"2022-09-28T10:00:00Z",
  "End":"2022-09-29T10:00:00Z"
}' "http://localhost:4000/users/1/schedules"

# Step 3: Trigger an incident
curl -d '{
  "Body":"An unexpected error happened on example-website.com on line 38. It needs addressing now!"
}' http://localhost:4000/incidents

# Step 4: Acknowledge the Incident
curl -X PUT "http://localhost:4000/incidents/1/acknowledge"
```

And if you don't acknowledge incoming incidents on Step 4, you will be reminded on Slack every 10 minutes:

![Being reminded on Slack about unacknowledged incidents](/assets/docs/incident-slack-reminder-example.png)

### Deploying to Encore Cloud

🥐 Simply push your changes up to your Git repository by running the following:

```bash
git add .
git commit -m "working implementation"
git push encore main
```

🥐 Then go to <https://app.encore.dev> and in a few minutes you should have your app deployed to the clouds!

### Architecture Diagram

Take a look at the [Encore Flow](/docs/develop/encore-flow) diagram that was automatically generated for our new application too!

![Being reminded on Slack about unacknowledged incidents](/assets/docs/incident-flow-diagram.png)

### GitHub Repository

🥐 Check out the `example-app-oncall` repository on GitHub for this example which includes additional features and tests:
<https://github.com/encoredev/example-app-oncall>

Alternatively, you can clone our example application by running this in your Terminal:

```bash
encore app create --example https://github.com/encoredev/example-app-oncall
```

### Feedback

🥐 We would love to hear what you learnt with this tutorial as well as what you're building.
Let us know by [tweeting your experience](https://twitter.com/encoredotdev) and maybe even posting your new app [to Show & Tell on our Community Forums](https://community.encore.dev/c/show-and-tell/12) for some friendly feedback?

