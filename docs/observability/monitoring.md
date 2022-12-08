---
seotitle: Monitoring your backend application
seodesc: See how you can monitor your backend application using Prometheus and Encore.
title: Monitoring
---

Encore provides built-in monitoring support using **Prometheus** to collect metrics every 10 seconds. This includes the large set of built-in metrics that Prometheus automatically collects, as well as: 

* The number of API calls, both in total and per endpoint
* A histogram of API call duration, per endpoint and response code

This is collected for all Encore environments and exposed through Encore’s cloud platform.

### Custom Metrics

We’re working on adding the ability to define custom metrics for monitoring application-specific behavior.