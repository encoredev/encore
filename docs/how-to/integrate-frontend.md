---
title: Integrate with a web frontend
---

When developing with Encore, one of the great parts is that calling API endpoints
is just as easy as calling a function, and a great developer experience with immediate
auto-completion and compile-time checking of the data you pass in.

However, calling APIs from another project like a website or a mobile app has traditionally been
a frustrating experience, involving lots of manual boilerplate to create type-safe client classes.

Not anymore. With Encore, you can generate a type-safe client to get all the same benefits for an external projects.

## Generating API clients

To generate a client, simply use `encore gen client --lang=<lang> app-id`, where `lang` is the language code.
For example, to generate a type-safe TypeScript client, use `--lang=typescript`.

The precise structure of the generated code depends on the language, to make sure it's idiomatic and easy to use,
but always includes all the publicly accessible endpoints, data structures, and documentation strings.

See `encore gen client --help` for a list of currently supported languages.