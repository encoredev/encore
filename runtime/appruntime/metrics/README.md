# `metrics`

This package allows an Encore app to export custom metrics to metric providers.

We have two types of custom metrics: a predefined set of custom metrics which are enabled for all Encore apps and
user-defined custom metrics.

## Predefined Encore metrics

These are the metrics all Encore apps export to metric providers:

- `e_requests_total` measures the number of requests and has three labels `service`, `endpoint` and `code`. `code` is a
  human-readable HTTP status code (e.g. `ok`, `not_found`).
- `e_sys_memory_heap_objects_bytes` measures the memory occupied by live objects and dead objects that have not yet been
  marked free by the garbage collector.
- `e_sys_sched_goroutines` measures the number of live goroutines.