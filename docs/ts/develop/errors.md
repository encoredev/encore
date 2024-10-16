---
seotitle: API Errors – Types, Wrappers, and Codes
seodesc: See how to return structured error information from your APIs using Encore's errs package, and how to build precise error messages for complex business logic.
title: API Errors
subtitle: Returning structured error information from your APIs
infobox: {
  title: "API Errors",
  import: "encore.dev/api",
}
lang: ts
---

Encore provides a standardized format of returning errors from API endpoints.

It looks like this:

```json
// HTTP 404 Not Found
{
    "code": "not_found",
    "message": "sprocket not found",
    "details": null
}
```

To return this, throw the `APIError` exception that Encore provides in the `encore.dev/api` module, with the appropriate error code:

```typescript
import { APIError, ErrCode } from "encore.dev/api";

throw new APIError(ErrCode.NotFound, "sprocket not found");

// or as a shorthand you can also write:
throw APIError.notFound("sprocket not found");
```


## Error Codes

The `ErrCode` type in the `encore.dev/api` module defines error codes for common error scenarios.
They are identical to the codes defined by `gRPC` for interoperability.

The table below summarizes the error codes.

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


## Additional details

To attach additional structured details to errors, use the `withDetails` method on an `APIError`. The details will be returned with the error to external clients.

