---
seotitle: Request validation in your backend application
seodesc: Learn how request validation works, and see how you can use Encore's built-in middleware to validate incoming requests in your backend application.
title: Validation
subtitle: Making sure everything's right in the world
lang: go
---

When receiving incoming requests it's best practice to validate the
payload to make sure it meets your expectations, contains all the necessary
fields, and so on.

Encore provides an out-of-the-box middleware that automatically validates
incoming requests if the request type implements the method `Validate() error`.

If it does, Encore will call this method after deserializing the request payload,
and only call your API handler (and other middleware) if the validation function
returns `nil`.

If the validation function returns an [`*errs.Error`](/docs/develop/errors) that error
is reported unmodified to the caller. Other errors are converted to an `*errs.Error`
with code `InvalidArgument`, which results in a HTTP response with status code `400 Bad Request`.

This design means that it's easy to use your validation library of choice.
In the future we're looking to provide an out-of-the-box validation library
for an even better developer experience.
