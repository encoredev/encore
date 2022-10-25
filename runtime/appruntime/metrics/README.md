# `metrics`

This package allows an Encore app to export custom metrics to cloud providers.

We have two types of custom metrics: a predefined set of custom metrics which are enabled for all Encore apps and
user-defined custom metrics. User-defined custom metrics aren't supported yet.

## Predefined Encore metrics

These are the metrics all Encore apps export to cloud providers:

- `e_requests_total` measures the number of requests and has two tags `service` and `endpoint`.
- `e_errors_total` measures the number of errors and has three tags `service`, `endpoint` and `code`. `code` is a
  human-readable error code (e.g. `not_found`).
- `e_request_durations_milliseconds` measures the response time and has three tags `service`, `endpoint`
  and `status_code`. `status_code` is an HTTP status code (e.g. `201`).