---
seotitle: Defining services with Encore.go
seodesc: Learn how to create microservices and define APIs for your cloud backend application using Go and Encore. The easiest way of building cloud backends.
title: Defining Services
subtitle: Simplifying (micro-)service development
lang: go
---

Encore.go makes it simple to build applications with one or many services, without needing to manually handle the typical complexity of developing microservices.

## Defining a service

With Encore.go you define a service by [defining at least one API](/docs/go/primitives/defining-apis) within a regular Go package. Encore recognizes this as a service, and uses the package name as the service name.

On disk it might look like this:

```
/my-app
├── encore.app          // ... and other top-level project files
│
├── hello               // hello service (a Go package)
│   ├── hello.go        // hello service code
│   └── hello_test.go   // tests for hello service
│
└── world               // world service (a Go package)
    └── world.go        // world service code
```


This means building a microservices architecture is as simple as creating multiple Go packages within your application.
See the [app structure documentation](/docs/go/primitives/app-structure) for more details.

<GitHubLink 
    href="https://github.com/encoredev/examples/tree/main/trello-clone" 
    desc="Simple microservices example application." 
/>

## Service Initialization

Under the hood Encore automatically generates a `main` function that initializes all your infrastructure resources when the application starts up. This means you don't write a `main` function for your Encore application.

If you want to customize the initialization behavior of your service, you can define a service struct and define custom initialization logic with that. See the [service struct docs](/docs/go/primitives/service-structs) for more info.
