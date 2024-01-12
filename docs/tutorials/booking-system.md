---
title: Building a Booking System
subtitle: Learn how to build your own appointment booking system with both user facing and admin functionality
seotitle: How to build an Appointment Booking System in Go
seodesc: Learn how to build an appointment booking tool using Go and Encore. Get your entire application running in the cloud in 30 minutes!
---

In this tutorial we'll build a booking system with a user facing UI (see available slots and book appointments) and an admin dashboard (manage scheduled appointments and set availability). You will learn how to:

* Create API endpoints using Encore (both public and authenticated).
* Working with PostgreSQL databases using [sqlc](https://sqlc.dev/) and [pgx](https://github.com/jackc/pgx).
* Scrub sensitive user data from traces.
* Work with dates and times in Go.
* Authenticate requests using an auth handler.
* Send emails using a SendGrid integration.

[Demo version of the app](https://prod-booking-system-teti.encr.app/frontend)

The final result will look like this:

<img className="w-full h-auto" src="/assets/tutorials/booking-system/user-calendar.png" title="User calendar" />

<img className="w-full h-auto" src="/assets/tutorials/booking-system/admin-dashboard.png" title="Admin dashboard" />

If you want to skip ahead you can view the final project here: [https://github.com/encoredev/examples/tree/main/booking-system](https://github.com/encoredev/examples/tree/main/booking-system)

## 1. Create your Encore application

<Callout type="info">

To make it easier to follow along, we've laid out a trail of croissants to guide your way.
Whenever you see a 🥐 it means there's something for you to do.

Make sure you have [Docker](https://docker.com) installed and running, it is used by Encore to run PostgreSQL databases locally.
</Callout>

🥐 Create a new Encore application, using this tutorial project's starting-point branch. This gives you a ready-to-go frontend to use.

```shell
$ encore app create booking-system --example=github.com/encoredev/example-booking-system/tree/starting-point
```


🥐 Check that your frontend works:

```shell
$ cd booking-system
$ encore run
```

Then visit [http://localhost:4000/frontend/](http://localhost:4000/frontend/) to see the frontend.
It won't function yet, since we haven't yet built the backend, so let's do just that!

When we're done we'll have a backend with this [architecture](/docs/observability/encore-flow):

<img className="w-full h-auto" src="/assets/tutorials/booking-system/booking-system-flow.png" title="Encore Flow" />

## 2. Create booking service

Let's start by creating the functionality to view bookable slots.

With Encore you define a service by [defining one or more APIs](/docs/primitives/services-and-apis#defining-apis) within a regular Go package. Encore recognizes this as a service, and uses the package name as the service name. When deploying, Encore will automatically [provision the required infrastructure](/docs/deploy/infra) for each service.

We already have a Go package named `booking`, let's turn that into an Encore service.    

🥐 Inside the `booking` folder, create a file named `slots.go`.

```shell
$ touch booking/slots.go
```

🥐 Add an Encore API endpoint named `GetBookableSlots` that takes a date as input. The endpoint will return a list of bookable slots from the supplied date and six days forward (so that we can show a week view calendar in the UI).

```go
-- booking/slots.go --
package booking

import (
	"context"
	"github.com/jackc/pgx/v5/pgtype"
	"time"
)

const DefaultBookingDuration = 1 * time.Hour

type BookableSlot struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

type SlotsParams struct{}

type SlotsResponse struct{ Slots []BookableSlot }

//encore:api public method=GET path=/slots/:from
func GetBookableSlots(ctx context.Context, from string) (*SlotsResponse, error) {
	fromDate, err := time.Parse("2006-01-02", from)
	if err != nil {
		return nil, err
	}

	const numDays = 7

	var slots []BookableSlot
	for i := 0; i < numDays; i++ {
		date := fromDate.AddDate(0, 0, i)
		daySlots, err := bookableSlotsForDay(date)
		if err != nil {
			return nil, err
		}
		slots = append(slots, daySlots...)
	}

	return &SlotsResponse{Slots: slots}, nil
}

func bookableSlotsForDay(date time.Time) ([]BookableSlot, error) {
	// 09:00
	availStartTime := pgtype.Time{
		Valid:        true,
		Microseconds: int64(9*3600) * 1e6,
	}
	// 17:00
	availEndTime := pgtype.Time{
		Valid:        true,
		Microseconds: int64(17*3600) * 1e6,
	}

	availStart := date.Add(time.Duration(availStartTime.Microseconds) * time.Microsecond)
	availEnd := date.Add(time.Duration(availEndTime.Microseconds) * time.Microsecond)

	// Compute the bookable slots in this day, based on availability.
	var slots []BookableSlot
	start := availStart
	for {
		end := start.Add(DefaultBookingDuration)
		if end.After(availEnd) {
			break
		}
		slots = append(slots, BookableSlot{
			Start: start,
			End:   end,
		})
		start = end
	}

	return slots, nil
}
```

The availability is currently hardcoded to be 09:00 - 17:00 for each day. Later we'll add the functionality to set it for each day of the week.
We are also returning time slots that have already passed. Don't worry, we'll come back and fix it later on.

🥐 Let's try it! Open up the Local Development Dashboard running at <http://localhost:9400> and try calling
the `booking.GetBookableSlots` endpoint, passing in `2024-12-01`.

If you prefer to use the terminal instead run `curl http://localhost:4000/slots/2024-12-01` in
a new terminal instead. Either way you should see the response:

```json
{
  "Slots": [
    {
      "start": "2024-12-01T09:00:00Z",
      "end": "2024-12-01T10:00:00Z"
    },
    {
      "start": "2024-12-01T10:00:00Z",
      "end": "2024-12-01T11:00:00Z"
    },
    {
      "start": "2024-12-01T11:00:00Z",
      "end": "2024-12-01T12:00:00Z"
    },
    ...
  ]
}
```

## 3. Book an appointment

Next, we want to make it possible to book an appointment. We'll need a database to store the bookings in. Encore makes it really simple to [create and use databases](/docs/primitives/databases) (both for local and cloud environments), but for this example we will also make use of [sqlc](https://sqlc.dev/) that will compile our SQL queries into type-safe Go code that we can use in our application.

🥐 Let's create a SQL database for our booking service and the required sqlc scaffolding. Create the following file structure:

```
/my-app
└── booking                              // booking service (a Go package)
    ├── db                               // (New) db related files (directory)
    │   ├── migrations                   // (New) db migrations (directory)
    │   │   └── 1_create_table.up.sql    // (New) db migration schema
    │   └── query.sql                    // (New) SQL queries
    ├── sqlc.yaml                        // (New) sqlc config file
    ├── slots.go                         // booking service code
    └── helpers.go                       // booking service code
```

🥐 Naming of the database migration file is important, it must look something like: `1_<name>.up.sql`.

Add the following contents to the migration file:

```sql
-- booking/db/migrations/1_create_tables.up.sql --
CREATE TABLE booking (
    id BIGSERIAL PRIMARY KEY,
    start_time TIMESTAMP NOT NULL,
    end_time TIMESTAMP NOT NULL,
    email TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);
```

🥐 Next, install the sqlc library:

```shell
$ go get github.com/sqlc-dev/sqlc/cmd/sqlc
```

🥐 Next, we need to configure sqlc. Add the following contents to `sqlc.yaml`:

```yaml
version: "2"
sql:
  - engine: "postgresql"
    queries: "db/query.sql"
    schema: "./db/migrations"
    gen:
      go:
        package: "db"
        out: "db"
        sql_package: "pgx/v5"
```

This instructs sqlc to generate Go code from the queries in `db/query.sql` and models from the schemas in the `db/migrations` folder.

🥐 Let's create our first SQL queries. Add the following contents to `db/query.sql`:

```sql
-- name: InsertBooking :one
INSERT INTO booking (start_time, end_time, email)
VALUES ($1, $2, $3)
RETURNING *;

-- name: ListBookingsBetween :many
SELECT * FROM booking
WHERE start_time >= $1 AND end_time <= $2;

-- name: ListBookings :many
SELECT * FROM booking;

-- name: DeleteBooking :exec
DELETE FROM booking WHERE id = $1;

```

🥐 It's time for sqlc to shine! Run the following command in your terminal:

```shell
$ cd booking
$ sqlc generate
```

Three files should now have been generated inside the `db` folder: `query.sql.go`, `db.go` and `models.go`. These files contain generated Go code and should not be manually edited. We will be adding more queries to `db/query.sql` later and then re-run `sqlc generate` to update the generated Go code.

Now let's create an endpoint that makes use of one of these queries.

🥐 Create `booking/booking.go` with the contents:

```go
-- booking/booking.go --
package booking

import (
	"context"
	"time"

	"encore.app/booking/db"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"encore.dev/beta/errs"
	"encore.dev/storage/sqldb"
)

var (
	bookingDB = sqldb.NewDatabase("booking", sqldb.DatabaseConfig{
		Migrations: "./db/migrations",
	})

	pgxdb = sqldb.Driver[*pgxpool.Pool](bookingDB)
	query = db.New(pgxdb)
)

type Booking struct {
	ID    int64     `json:"id"`
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
	Email string    `encore:"sensitive"`
}

type BookParams struct {
	Start time.Time `json:"start"`
	Email string    `encore:"sensitive"`
}

//encore:api public method=POST path=/booking
func Book(ctx context.Context, p *BookParams) error {
	eb := errs.B()

	now := time.Now()
	if p.Start.Before(now) {
		return eb.Code(errs.InvalidArgument).Msg("start time must be in the future").Err()
	}

	tx, err := pgxdb.Begin(ctx)
	if err != nil {
		return eb.Cause(err).Code(errs.Unavailable).Msg("failed to start transaction").Err()
	}
	defer tx.Rollback(context.Background()) // committed explicitly on success
	
	_, err = query.InsertBooking(ctx, db.InsertBookingParams{
		StartTime: pgtype.Timestamp{Time: p.Start, Valid: true},
		EndTime:   pgtype.Timestamp{Time: p.Start.Add(DefaultBookingDuration), Valid: true},
		Email:     p.Email,
	})
	if err != nil {
		return eb.Cause(err).Code(errs.Unavailable).Msg("failed to insert booking").Err()
	}

	if err := tx.Commit(ctx); err != nil {
		return eb.Cause(err).Code(errs.Unavailable).Msg("failed to commit transaction").Err()
	}
	return nil
}
```

We are now using the generated type-safe `query.InsertBooking` function to make the database operation.

Notice the `encore:"sensitive"` tag on the `Email` field. This tells Encore to scrub this field so that the data is not viewable in the traces for deployed environments. This is useful for fields that contain [sensitive data](/docs/develop/api-schemas#sensitive-data) such as email addresses, passwords, etc.

🥐 Restart `encore run` to cause the database to be created, and then call the `booking.Book` endpoint:

```shell
$ curl -X POST 'http://localhost:4000/booking' -d '{"start": "2024-12-11T09:00:00Z", "email": "test@example.com"}'
```

Congratulations, you have now booked your first appointment!

## 4. Authentication

To provide an admin dashboard for our booking system, we need to add authentication to our application so that we can have protected endpoints.

Keep in mind, in this tutorial we'll only include a very basic implementation.

🥐 Let's start by creating a new service named `user`:

```shell
$ mkdir user
$ touch user/auth.go
```

🥐 Add the following contents to `user/auth.go`:

```go
-- user/auth.go --
package user

import (
	"context"
	"encore.dev/beta/auth"
	"encore.dev/beta/errs"
)

type Data struct {
	Email string
}

type AuthParams struct {
	Authorization string `header:"Authorization"`
}

//encore:authhandler
func AuthHandler(ctx context.Context, p *AuthParams) (auth.UID, *Data, error) {
	if p.Authorization != "" {
		return "test", &Data{}, nil
	}
	return "", nil, errs.B().Code(errs.Unauthenticated).Msg("no auth header").Err()
}

```

This function is our [auth handler](/docs/develop/auth#the-auth-handler). An Encore applications can designate a special function to handle authentication, 
by defining a function and annotating it with `//encore:authhandler`. This annotation tells Encore to run the function whenever an 
incoming API call contains authentication data.

The auth handler is responsible for validating the incoming authentication data and returning an `auth.UID` (a string type representing a user id). 
The `auth.UID` can be whatever you wish, but in practice it usually maps directly to the primary key stored in a user table (either defined in the Encore service or in an external service like Firebase or Okta).

In order to keep this example simple, we'll just approve any request containing a token that is not empty.

Next we will implement some of our auth endpoints and make use of our newly created auth handler.

## 5. Setting availability

Right now the availability is hardcoded to 9:00 - 17:00. Let's add the functionality to let our admin users customize this. 

Let's start by adding another migration file, this time to create an `avilability` table.

🥐 Create a file called `2_add_availability.up.sql` inside the `booking/db/migrations` folder. Add the following contents to that file:

```sql
-- booking/db/migrations/2_add_availability.up.sql --
CREATE TABLE availability (
    weekday SMALLINT NOT NULL PRIMARY KEY, -- Sunday=0, Monday=1, etc.
    start_time TIME NULL, -- null indicates not available
    end_time TIME NULL -- null indicates not available
);

-- Add some placeholder availability to get started
INSERT INTO availability (weekday, start_time, end_time) VALUES
    (0, '09:30', '17:00'),
    (1, '09:00', '17:00'),
    (2, '09:00', '18:00'),
    (3, '08:30', '18:00'),
    (4, '09:00', '17:00'),
    (5, '09:00', '17:00'),
    (6, '09:30', '16:30');
```

🥐 We can now add two queries to `booking/db/query.sql` so that we can store and retrieve availability:

```sql
-- booking/db/query.sql --
-- name: GetAvailability :many
SELECT * FROM availability
ORDER BY weekday;

-- name: UpdateAvailability :exec
INSERT INTO availability (weekday, start_time, end_time)
VALUES (@weekday, @start_time, @end_time)
ON CONFLICT (weekday) DO UPDATE
SET start_time = @start_time, end_time = @end_time;
```

🥐 Run `sqlc generate` to update the generated Go code.

🥐 Create a new file in the `booking` service named `availability.go`:

```shell
$ touch booking/availability.go
```

🥐 Add the following to that file:

```go
-- booking/availability.go --
package booking

import (
	"context"
	"errors"
	"fmt"

	"encore.app/booking/db"
	"github.com/jackc/pgx/v5/pgtype"

	"encore.dev/beta/errs"
	"encore.dev/rlog"
)

type Availability struct {
	Start *string `json:"start" encore:"optional"`
	End   *string `json:"end" encore:"optional"`
}

type GetAvailabilityResponse struct {
	Availability []Availability
}

//encore:api public method=GET path=/availability
func GetAvailability(ctx context.Context) (*GetAvailabilityResponse, error) {
	rows, err := query.GetAvailability(ctx)
	if err != nil {
		return nil, err
	}

	availability := make([]Availability, 7)
	for _, row := range rows {
		day := row.Weekday
		if day < 0 || day > 6 {
			rlog.Error("invalid week day in availability table", "row", row)
			continue
		}

		// These never fail
		start, _ := row.StartTime.TimeValue()
		end, _ := row.EndTime.TimeValue()
		availability[day] = Availability{
			Start: timeToStr(start),
			End:   timeToStr(end),
		}
	}

	return &GetAvailabilityResponse{Availability: availability}, nil
}

type SetAvailabilityParams struct {
	Availability []Availability
}

//encore:api auth method=POST path=/availability
func SetAvailability(ctx context.Context, params SetAvailabilityParams) error {
	eb := errs.B()
	tx, err := pgxdb.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background()) // committed explicitly on success

	qry := query.WithTx(tx)
	for weekday, a := range params.Availability {
		if weekday > 6 {
			return eb.Code(errs.InvalidArgument).Msgf("invalid weekday %d", weekday).Err()
		}

		start, err1 := strToTime(a.Start)
		end, err2 := strToTime(a.End)
		if err := errors.Join(err1, err2); err != nil {
			return eb.Cause(err).Code(errs.InvalidArgument).Msg("invalid start/end time").Err()
		} else if start.Valid != end.Valid {
			return eb.Code(errs.InvalidArgument).Msg("both start/stop must be set, or both null").Err()
		} else if start.Valid && start.Microseconds > end.Microseconds {
			return eb.Code(errs.InvalidArgument).Msg("start must be before end").Err()
		}

		err = qry.UpdateAvailability(ctx, db.UpdateAvailabilityParams{
			Weekday:   int16(weekday),
			StartTime: start,
			EndTime:   end,
		})
		if err != nil {
			return eb.Cause(err).Code(errs.Unavailable).Msg("failed to update availability").Err()
		}
	}

	err = tx.Commit(ctx)
	return errs.WrapCode(err, errs.Unavailable, "failed to commit transaction")
}
```

This file contains two endpoints, a setter and a getter. The `SetAvailability` endpoint is protected by the `auth` middleware which means that the user must be authenticated in order to call it. The `GetAvailability` endpoint is public and can be called without authentication.

🥐 Let's set the availability for each day of the week. Open the Development Dashboard at <http://localhost:9400> and select the `booking.SetAvailability` endpoint in the API Explorer. For the request body, paste the following:

```json
{
    "Availability": [{
        "start": "09:30",
        "end": "17:00"
    },{
        "start": "09:00",
        "end": "17:00"
    },{
        "start": "09:00",
        "end": "18:00"
    },{
        "start": "08:30",
        "end": "18:00"
    },{
        "start": "09:00",
        "end": "17:00"
    },{
        "start": "09:00",
        "end": "17:00"
    },{
        "start": "09:30",
        "end": "16:30"
    }]
}
```

<Callout type="info">

Don't leave the auth token empty, it will cause the auth handler to reject the request. You can use any value for the auth token.

</Callout>

Now try retrieving the availability by calling the `booking.GetAvailability` endpoint through the API Explorer in the Development Dashboard.

🥐 Somewhere inside the `booking` package, add the following functions and import the `slices` package:

```go
func listBookingsBetween(
	ctx context.Context,
	start, end time.Time,
) ([]*Booking, error) {
	rows, err := query.ListBookingsBetween(ctx, db.ListBookingsBetweenParams{
		StartTime: pgtype.Timestamp{Time: start, Valid: true},
		EndTime:   pgtype.Timestamp{Time: end, Valid: true},
	})
	if err != nil {
		return nil, err
	}
	var bookings []*Booking
	for _, row := range rows {
		bookings = append(bookings, &Booking{
			ID:    row.ID,
			Start: row.StartTime.Time,
			End:   row.EndTime.Time,
			Email: row.Email,
		})
	}
	return bookings, nil
}

func filterBookableSlots(
	slots []BookableSlot,
	now time.Time, 
	bookings []*Booking,
) []BookableSlot {
	// Remove slots for which the start time has already passed.
	slots = slices.DeleteFunc(slots, func(s BookableSlot) bool {
		// Has the slot already passed?
		if s.Start.Before(now) {
			return true
		}

		// Is there a booking that overlaps with this slot?
		for _, b := range bookings {
			if b.Start.Before(s.End) && b.End.After(s.Start) {
				return true
			}
		}

		return false
	})
	return slots
}
```

We will use these functions to figure out which slots are bookable and which are not to avoid double bookings. 

🥐 Now we can update the `Book` endpoint inside `booking.go` and make use of these new functions:

```go
HL booking/booking.go 15:27
-- booking/booking.go --
//encore:api public method=POST path=/booking
func Book(ctx context.Context, p *BookParams) error {
	eb := errs.B()

	now := time.Now()
	if p.Start.Before(now) {
		return eb.Code(errs.InvalidArgument).Msg("start time must be in the future").Err()
	}

	tx, err := pgxdb.Begin(ctx)
	if err != nil {
		return eb.Cause(err).Code(errs.Unavailable).Msg("failed to start transaction").Err()
	}
	defer tx.Rollback(context.Background()) // committed explicitly on success

	// Get the bookings for this day.
	startOfDay := time.Date(p.Start.Year(), p.Start.Month(), p.Start.Day(), 0, 0, 0, 0, p.Start.Location())
	bookings, err := listBookingsBetween(ctx, startOfDay, startOfDay.AddDate(0, 0, 1))
	if err != nil {
		return eb.Cause(err).Code(errs.Unavailable).Msg("failed to list bookings").Err()
	}

	// Is this slot bookable?
	slot := BookableSlot{Start: p.Start, End: p.Start.Add(DefaultBookingDuration)}
	if len(filterBookableSlots([]BookableSlot{slot}, now, bookings)) == 0 {
		return eb.Code(errs.InvalidArgument).Msg("slot is unavailable").Err()
	}

	_, err = query.InsertBooking(ctx, db.InsertBookingParams{
		StartTime: pgtype.Timestamp{Time: p.Start, Valid: true},
		EndTime:   pgtype.Timestamp{Time: p.Start.Add(DefaultBookingDuration), Valid: true},
		Email:     p.Email,
	})
	if err != nil {
		return eb.Cause(err).Code(errs.Unavailable).Msg("failed to insert booking").Err()
	}

	if err := tx.Commit(ctx); err != nil {
		return eb.Cause(err).Code(errs.Unavailable).Msg("failed to commit transaction").Err()
	}
	return nil
}
```

🥐 Inside `slots.go`, update the `GetBookableSlots` endpoint and the `bookableSlotsForDay` functions to look like this:

```go
HL booking/slots.go 7:12
HL booking/slots.go 18:23
HL booking/slots.go 29:36
HL booking/slots.go 39:48
-- booking/slots.go --
//encore:api public method=GET path=/slots/:from
func GetBookableSlots(ctx context.Context, from string) (*SlotsResponse, error) {
	fromDate, err := time.Parse("2006-01-02", from)
	if err != nil {
		return nil, err
	}

	availabilityResp, err := GetAvailability(ctx)
	if err != nil {
		return nil, err
	}
	availability := availabilityResp.Availability

	const numDays = 7

	var slots []BookableSlot
	for i := 0; i < numDays; i++ {
		date := fromDate.AddDate(0, 0, i)
		weekday := int(date.Weekday())
		if len(availability) <= weekday {
			break
		}
		daySlots, err := bookableSlotsForDay(date, &availability[weekday])
		if err != nil {
			return nil, err
		}
		slots = append(slots, daySlots...)
	}

	// Get bookings for the next 7 days.
	activeBookings, err := listBookingsBetween(ctx, fromDate, fromDate.AddDate(0, 0, numDays))
	if err != nil {
		return nil, err
	}

	slots = filterBookableSlots(slots, time.Now(), activeBookings)
	return &SlotsResponse{Slots: slots}, nil
}

func bookableSlotsForDay(date time.Time, avail *Availability) ([]BookableSlot, error) {
	if avail.Start == nil || avail.End == nil {
		return nil, nil
	}
	availStartTime, err1 := strToTime(avail.Start)
	availEndTime, err2 := strToTime(avail.End)
	if err := errors.Join(err1, err2); err != nil {
		return nil, err
	}

	availStart := date.Add(time.Duration(availStartTime.Microseconds) * time.Microsecond)
	availEnd := date.Add(time.Duration(availEndTime.Microseconds) * time.Microsecond)

	// Compute the bookable slots in this day, based on availability.
	var slots []BookableSlot
	start := availStart
	for {
		end := start.Add(DefaultBookingDuration)
		if end.After(availEnd) {
			break
		}
		slots = append(slots, BookableSlot{
			Start: start,
			End:   end,
		})
		start = end
	}

	return slots, nil
}
```

## 6. Managing scheduled bookings

In order to display the scheduled bookings in the admin dashboard we need to add the functionality to list all bookings. While we're at it, we will also make it possible to delete bookings.

🥐 Add two new endpoints to `booking/booking.go`:

```go
-- booking/booking.go --
type ListBookingsResponse struct {
	Booking []*Booking `json:"bookings"`
}

//encore:api auth method=GET path=/booking
func ListBookings(ctx context.Context) (*ListBookingsResponse, error) {
	rows, err := query.ListBookings(ctx)
	if err != nil {
		return nil, err
	}

	var bookings []*Booking
	for _, row := range rows {
		bookings = append(bookings, &Booking{
			ID:    row.ID,
			Start: row.StartTime.Time,
			End:   row.EndTime.Time,
			Email: row.Email,
		})
	}
	return &ListBookingsResponse{Booking: bookings}, nil
}

//encore:api auth method=DELETE path=/booking/:id
func DeleteBooking(ctx context.Context, id int64) error {
	return query.DeleteBooking(ctx, id)
}
```

That's it! We now have all the backend endpoints in place to be able to supply the frontend with data. 🎉

## 7. Running the React frontend

The frontend should now be working as expected.

🥐 Go to [http://localhost:4000/frontend/](http://localhost:4000/frontend/) and try out your new booking system.

The frontend is built using [React](https://react.dev/) and [Tailwind CSS](https://tailwindcss.com/). It uses Encore's ability to generate type-safe [request clients](https://encore.dev/docs/develop/client-generation). This means you don't need to manually keep the request/response objects in sync on the frontend. To generate a client:

```bash
$ encore gen client <APP_NAME> --output=./src/client.ts --env=<ENV_NAME>
```

While you're developing, you are going to want to run this command quite often (whenever you make a change to your endpoints) so having it as an `npm` script is a good idea. Take a look at the scripts in the `package.json` file:

```json
{
...
"scripts": {
    ...
    "gen": "encore gen client <Encore App ID> --output=./src/lib/client.ts --env=staging",
    "gen:local": "encore gen client <Encore App ID> --output=./src/lib/client.ts --env=local"
  },
}
```

For this frontend we use the request client together with [TanStack Query](https://tanstack.com/query/latest). When building something a bit more complex, you will likely need to deal with caching, refetching, and data going stale. [TanStack Query](https://tanstack.com/query/latest) is a popular library that was built to solve exactly these problems and works great with the Encore request client. 

See our the docs page about [integrating with a web frontend](docs/how-to/integrate-frontend) to learn more. 

## 8. Deploy to Encore's development cloud

Let's deploy the project to Encore's free development cloud.

Encore comes with built-in CI/CD, and the deployment process is as simple as a `git push`.
(You can also integrate with GitHub to activate per Pull Request Preview Environments, learn more in the [CI/CD docs](/docs/deploy/deploying).)

🥐 Now, let's deploy your app to Encore's free development cloud by running:

```shell
$ git add -A .
$ git commit -m 'Initial commit'
$ git push encore
```

Encore will now build and test your app, provision the needed infrastructure, and deploy your application to the cloud.

After triggering the deployment, you will see a URL where you can view its progress in Encore's [Cloud Dashboard](https://app.encore.dev). It will look something like: `https://app.encore.dev/$APP_ID/deploys/...`

From there you can also see metrics, traces, link your app to a GitHub repo to get automatic deploys on new commits, and connect your own AWS or GCP account to use for production deployment.

🥐 When the deploy has finished, you can try out your booking system by going to `https://staging-$APP_ID.encr.app/frontend/`.

*You now have an Appointment Booking System running in the cloud, well done!*

## 8. Sending confirmation emails using SendGrid

In order for the users to get a confirmation email when they book an appointment we need to add an email integration. 

Conveniently for us, there is a ready to use SendGrid integration as an [Encore Bit](https://github.com/encoredev/examples?tab=readme-ov-file#bits). 

🥐 [Follow the instructions](https://github.com/encoredev/examples/tree/main/bits/sendgrid) to add the SendGrid integration to your project.

Next, we need to call our new `sendgrid` service when an appointment is booked.

🥐 Add a call to `sendgrid.Send` in the `Book` endpoint:

```go
HL booking/booking.go 41:59
-- booking/booking.go --
//encore:api public method=POST path=/booking
func Book(ctx context.Context, p *BookParams) error {
	eb := errs.B()

	now := time.Now()
	if p.Start.Before(now) {
		return eb.Code(errs.InvalidArgument).Msg("start time must be in the future").Err()
	}

	tx, err := pgxdb.Begin(ctx)
	if err != nil {
		return eb.Cause(err).Code(errs.Unavailable).Msg("failed to start transaction").Err()
	}
	defer tx.Rollback(context.Background()) // committed explicitly on success

	// Get the bookings for this day.
	startOfDay := time.Date(p.Start.Year(), p.Start.Month(), p.Start.Day(), 0, 0, 0, 0, p.Start.Location())
	bookings, err := listBookingsBetween(ctx, startOfDay, startOfDay.AddDate(0, 0, 1))
	if err != nil {
		return eb.Cause(err).Code(errs.Unavailable).Msg("failed to list bookings").Err()
	}

	// Is this slot bookable?
	slot := BookableSlot{Start: p.Start, End: p.Start.Add(DefaultBookingDuration)}
	if len(filterBookableSlots([]BookableSlot{slot}, now, bookings)) == 0 {
		return eb.Code(errs.InvalidArgument).Msg("slot is unavailable").Err()
	}

	_, err = query.InsertBooking(ctx, db.InsertBookingParams{
		StartTime: pgtype.Timestamp{Time: p.Start, Valid: true},
		EndTime:   pgtype.Timestamp{Time: p.Start.Add(DefaultBookingDuration), Valid: true},
		Email:     p.Email,
	})
	if err != nil {
		return eb.Cause(err).Code(errs.Unavailable).Msg("failed to insert booking").Err()
	}

	if err := tx.Commit(ctx); err != nil {
		return eb.Cause(err).Code(errs.Unavailable).Msg("failed to commit transaction").Err()
	}

	// Send confirmation email using SendGrid
	formattedTime := pgtype.Timestamp{Time: p.Start, Valid: true}.Time.Format("2006-01-02 15:04")
	_, err = sendgrid.Send(ctx, &sendgrid.SendParams{
		From: sendgrid.Address{
			Name:  "<your name>",
			Email: "<your email>",
		},
		To: sendgrid.Address{
			Email: p.Email,
		},
		Subject: "Booking Confirmation",
		Text:    "Thank you for your booking!\nWe look forward to seeing you soon at " + formattedTime,
		Html:    "",
	})

	if err != nil {
		return err
	}

	return nil
}
```

<Callout type="info">

The `From` email used when sending emails needs to go through the SendGrid verification process before it can be used. You can read more about it here: https://sendgrid.com/docs/ui/sending-email/sender-verification/

The default behaviour of the SendGrid integration is to only send emails on production environments. You can create production environments through the Encore Cloud Dashboard.

</Callout>

## 9. Deploy your finished Booking System

Now you're ready to deploy your finished Booking System, complete with a SendGrid integration.

🥐 As before, deploying your app to the cloud is as simple as running:

```shell
$ git add -A .
$ git commit -m 'Add sendgrid integration'
$ git push encore
```

### Celebrate with fireworks

Now that your app is running in the cloud, let's celebrate with some fireworks:

🥐 In the Cloud Dashboard, open the Command Menu by pressing **Cmd + K** (Mac) or **Ctrl + K** (Windows/Linux).

_From here you can easily access all Cloud Dashboard features and for example jump straight to specific services in the Service Catalog or view Traces for specific endpoints._

🥐 Type `fireworks` in the Command Menu and press enter. Sit back and enjoy the show!

![Fireworks](/assets/docs/fireworks.jpg)
