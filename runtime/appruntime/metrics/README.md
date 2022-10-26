# `metrics`

This package allows an Encore app to export custom metrics to cloud providers.

We have two types of custom metrics: a predefined set of custom metrics which are enabled for all Encore apps and
user-defined custom metrics. User-defined custom metrics aren't supported yet.

## Predefined Encore metrics

These are the metrics all Encore apps export to cloud providers:

- `e_requests_total` measures the number of requests and has three tags `service`, `endpoint` and `code`. `code` is a
  human-readable HTTP status code (e.g. `ok`, `not_found`).
- `e_request_duration_seconds` measures the response time in seconds and has three tags `service`, `endpoint` and
  `code`. `code` is a human-readable HTTP status code (e.g. `ok`, `not_found`).