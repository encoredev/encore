---
title: API Errors
subtitle: When the happy path turns sad
---

Encore supports returning structured error information from your APIs.

The key piece is the [encore.dev/beta/errs](https://pkg.go.dev/encore.dev/beta/errs) package.

## The errs.Error type

Structured errors are represented by the `errs.Error` type:

```go
type Error struct {
	// Code is the error code to return.
	Code ErrCode `json:"code"`
	// Message is a descriptive message of the error.
	Message string `json:"message"`
	// Details are user-defined additional details.
	Details ErrDetails `json:"details"`
	// Meta are arbitrary key-value pairs for use within
	// the Encore application. They are not exposed to external clients.
	Meta Metadata `json:"-"`
}
```

Returning an `*errs.Error` from an Encore API endpoint will result in Encore
serializing this struct to JSON and returning it in the response. Additionally
Encore will set the HTTP status code to match the error code (see the mapping table below).

For example:
```go
return &errs.Error{
	Code: errs.NotFound,
	Message: "sprocket not found",
}
```

Causes Encore to respond with a `HTTP 404` error with body:
```json
{
    "code": "not_found",
    "message": "sprocket not found",
    "details": null
}
```

## Error Wrapping

Encore applications are encouraged to always use the `errs` package to
manipulate errors. It supports wrapping errors to gradually add more error
information, and lets you easily define both structured error details to return
to external clients, as well as internal, key-value metadata for debugging
and error handling.

```go
func Wrap(err error, msg string, metaPairs ...interface{}) error
```
Use `errs.Wrap` to conveniently wrap an error, adding additional context and converting it to an `*errs.Error`.
If `err` is nil it returns `nil`. If `err` is already an `*errs.Error` it copies the Code, Details, and Meta fields over.

The variadic `metaPairs` parameter must be key-value pairs, where the key is always a `string` and the value can be
any built-in type. Existing key-value pairs from the `err` are merged into the new `*Error`.

```go
func WrapCode(err error, code ErrCode, msg string, metaPairs ...interface{}) error
```
`errs.WrapCode` is like `errs.Wrap` but also sets the error code.

```go
func Convert(err error) error
```
`errs.Convert` converts an error to an `*errs.Error`. If the error is already an `*errs.Error` it returns it unmodified.
If `err` is nil it returns nil.

## Error Codes

The `errs` package defines error codes for common error scenarios.
They are identical to the codes defined by `gRPC` for interoperability.

The table below summarizes the error codes.
You can find additional documentation about when to use them in the
[package documentation](https://pkg.go.dev/encore.dev/beta/errs#ErrCode).

| Code                  | String                  | HTTP Status               |
|-----------------------|-------------------------|---------------------------|
| `OK`                  | `"ok"`                  | 200 OK                    |
| `Canceled`            | `"canceled"`            | 499 Client Closed Request |
| `Unknown`             | `"unknown"`             | 500 Internal Server Error |
| `InvalidArgument`     | `"invalid_argument"`    | 400 Bad Request           |
| `DeadlineExceeded`    | `"deadline_exceeded"`   | 504 Gateway Timeout       |
| `NotFound`            | `"not_found"`           | 404 Not Found             |
| `AlreadyExists`       | `"already_exists"`      | 409 Conflict              |
| `PermissionDenied`    | `"permission_denied"`   | 403 Forbidden             |
| `ResourceExhausted`   | `"resource_exhausted"`  | 429 Too Many Requests     |
| `FailedPrecondition`  | `"failed_precondition"` | 400 Bad Request           |
| `Aborted`             | `"aborted"`             | 409 Conflict              |
| `OutOfRange`          | `"out_of_range"`        | 400 Bad Request           |
| `Unimplemented`       | `"unimplemented"`       | 501 Not Implemented       |
| `Internal`            | `"internal"`            | 500 Internal Server Error |
| `Unavailable`         | `"unavailable"`         | 503 Unavailable           |
| `DataLoss`            | `"data_loss"`           | 500 Internal Server Error |
| `Unauthenticated`     | `"unauthenticated"`     | 401 Unauthorized          |

## Error Building

In cases where you have complex business logic, or multiple error returns,
it's convenient to gradually add metadata to your error.

For this purpose Encore provides `errs.Builder`. The builder lets you
gradually set aspects of the error, using a chaining API design.
Use `errs.B()` to get a new builder that you can start chaining with directly.

When you want to return the constructed error call the `.Err() `method.

For example:

```go
func getBoard(ctx context.Context, boardID int64) (*Board, error) {
    // Construct a new error builder with errs.B()
	eb := errs.B().Meta("board_id", params.ID)

	b := &Board{ID: params.ID}
	err := sqldb.QueryRow(ctx, `
		SELECT name, created
		FROM board
		WHERE id = $1
	`, params.ID).Scan(&b.Name, &b.Created)
	if errors.Is(err, sqldb.ErrNoRows) {
        // Return a "board not found" error with code == NotFound
		return nil, eb.Code(errs.NotFound).Msg("board not found").Err()
	} else if err != nil {
        // Return a general error
		return nil, eb.Cause(err).Msg("could not get board").Err()
	}
    // ...
}
```

## Inspecting API Errors

When you call another API within Encore, the returned errors are always wrapped in `*errs.Error`.

You can inspect the error information either by casting to `*errs.Error`, or using the below
helper methods.

```go
func Code(err error) ErrCode
```
`errs.Code` returns the error code. If the error was not an `*errs.Error` it returns `errs.Unknown`.

```go
func Meta(err error) Metadata
type Metadata map[string]interface{}
```
`errs.Meta` returns any structured metadata present in the error. If the error was not an `*errs.Error` it returns nil.
Unlike when you return error information to external clients,
all the metadata is sent to the calling service, making debugging even easier.

```go
func Details(err error) ErrDetails
```
`errs.Details` returns the structured error details. If the error was not an `*errs.Error` or the error lacked details,
it returns nil.