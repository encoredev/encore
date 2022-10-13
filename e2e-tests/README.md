# End to End Tests

This folder contains end to end tests for Encore, which test everything in Encore
as an end user would use it.

The tests will:
1. Effectively run `encore run` on the [echo test app](./testdata/echo)
2. Perform some basic requests against the running app to verify behaviour
3. Generate the front end clients for the app
4. Run tests against using generated clients against the running app
5. Shutdown the running app
