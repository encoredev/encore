---
seotitle: Create a REST API with Ksql and Encore
seodesc: See how you can use Ksql  in your Encore application.
title: Create a REST API with Ksql and Encore
---

[Ksql](https://github.com/VinGarcia/ksql) is a popular tool for managing database

Encore provides excellent support for using them together to easily manage database schemas and migrations.
When you use Ksql, Encore will automatically create a database for you and run your migrations.


## Setting up Ksql
First you need to install Ksql in your machine, you can follow the instructions in the [Ksql](https://github.com/VinGarcia/ksql) repository.

Then, in the service that you want to use Ksql for, add the `*ksql.DB` as a dependency in your service struct (create a service struct if you don't already have one).

```sh 
go get  github.com/VinGarcia/ksql
go get github.com/vingarcia/ksql/adapters/kpostgres

```

## Create a pkg with the ksql connection

When use ksql you need use tag in your struct, see the example below

```go 
//contracts.go
package ksqldb

import "time"

type User struct {
	ID        int64     `json:"id" ksql:"id"`
	UserName  string    `json:"name" ksql:"username"`
	Password  string    `json:"password" ksql:"password"`
	Email     string    `json:"email" ksql:"email"`
	CreatedAt time.Time `json:"created_at" ksql:"created_at"`
}


```

Now create the connection with ksql in your pkg

```go
//database.go
package ksqldb

import (
	"encore.dev/storage/sqldb"
	"github.com/vingarcia/ksql"
	"github.com/vingarcia/ksql/adapters/kpostgres"
	"go4.org/syncutil"
)

var _ = sqldb.NewDatabase("orders", sqldb.DatabaseConfig{
	Migrations: "./migrations",
})

// Get returns a database connection  to the database.
// It is lazily created on first use.
func Get(orderdb *sqldb.Database) (ksql.DB, error) {
	// Attempt to setup the database connection pool if it hasn't
	// already been successfully setup.
	err := once.Do(func() error {
		var err error
		db, err = setup(orderdb)
		return err
	})

	return db, err
}

var (
	// once is like sync.Once except it re-arms itself on failure
	once syncutil.Once
	// db is the database connection
	db ksql.DB
)

// setup attempts to set up a database connection pool.
func setup(orderdb *sqldb.Database) (ksql.DB, error) {
	return kpostgres.NewFromSQLDB(orderdb.Stdlib())
}
```

## Using the ksql connection in your service
```go
package users

import (
	"context"

	"encore.app/pkg/ksqldb"
	"encore.dev/storage/sqldb"
	"github.com/vingarcia/ksql"
)

var usersdb = sqldb.Named("orders")
var table = ksql.NewTable("users")

//encore:service
type Service struct {
	db *ksql.DB
}

func initService() (*Service, error) {

	db, err := ksqldb.Get(usersdb)

	if err != nil {
		return nil, err
	}

	return &Service{
		db: &db,
	}, nil
}

// SAve users
func (s *Service) Save(ctx context.Context, user ksqldb.User) (*ksqldb.User, error) {

	err := s.db.Insert(ctx, table, &user)

	if err != nil {
		return nil, err
	}

	return &user, nil
}

// Get all userss
func (s *Service) GetAll(ctx context.Context) (*[]ksqldb.User, error) {

	var users []ksqldb.User

	err := s.db.Query(ctx, &users, "SELECT * FROM users")

	if err != nil {
		return nil, err
	}

	return &users, nil
}

```

Afer that you can use the service in your api

```go
package users

import (
	"context"
	"fmt"

	"encore.app/pkg/ksqldb"
	"encore.dev/beta/errs"
	"encore.dev/rlog"
)

type Response struct {
	Data string
}


//encore:api public method=POST path=/user
func (s *Service) CreateUser(ctx context.Context, request Request) (Response, error) {

	if request.Name == "" || request.Email == "" || request.Password == "" {
		return Response{}, &errs.Error{
			Code:    errs.InvalidArgument,
			Message: "Name, email and password are required",
		}
	}

	_, err := s.Save(ctx, ksqldb.User{
		UserName: request.Name,
		Email:    request.Email,
		Password: request.Password,
	})

	if err != nil {
		rlog.Error(fmt.Sprintf("Error to save user %v", err))
		return Response{}, &errs.Error{
			Code:    errs.InvalidArgument,
			Message: "Error to save user",
		}
	}
	return Response{
		Data: "Usuario criado com sucesso",
	}, nil
}

//encore:api public method=GET path=/users
func (s *Service) GetUsers(ctx context.Context) (*[]ksqldb.User, error) {

	users, err := s.GetAll(ctx)

	if err != nil {
		return nil, &errs.Error{
			Code:    errs.InvalidArgument,
			Message: "Error to get users",
		}
	}

	return users, nil
}
```