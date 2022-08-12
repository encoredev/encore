---
title: App Structure
subtitle: Structuring your Encore application and its systems
---

When you're building with Encore, it's usually best to use *one Encore application* for your entire project.
This lets Encore build an application model that spans your entire application, which is necessary for features like distributed tracing to work.

You can use separate subfolders and packages to create a logical separation between the different major systems in your project.

As an example, a Trello app might consist of three systems: the **Trello** system (for managing trello boards & cards),
the **User** system (for user and organization management, and authentication), and the **Premium** system (for subscriptions
and paid features).

On disk it might look like this:

```go
/my-trello-clone
├── encore.app       // ... and other top-level project files
│
├── premium          // premium system
│   ├── payment      // payment service
│   └── subscription // subscription service
│
├── trello           // trello system
│   ├── board        // board service
│   └── card         // card service
│
└── usr              // user system
    ├── org          // org service
    └── user         // user service
```

Each service consists of Go files that implement the service business logic,
database migrations for defining the structure of the database(s), and so on.
