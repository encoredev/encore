---
title: App Structure
subtitle: Structuring your Encore application and its systems
---

When you're building with Encore, it's usually best to use *one Encore application* for your entire project.
This lets Encore build an application model that spans your entire application, which is necessary for features like
distributed tracing to work.

## Structuring your app around services
Services are the base building blocks of your application, they separate your application in isolated units with their
own endpoints and database (if using one). In a micro-services application, an Encore service represents a single
micro-service. While services are isolated and each have their own databases by default,
[databases may be shared between services](/docs/how-to/share-db-between-services).

As an example, a Trello app might consist of 6 services: the **Board** service, the **Card** service, the
**Organization** service, the **User** service, the **Payment** service, and the **Subscription** service. The **User**
and **Organization** might share their database for foreign keys purposes.

On disk it might look like this:

```go
/my-trello-clone
├── encore.app       // ... and other top-level project files
│
├── payment      // payment service
├── subscription // subscription service
│
├── board        // board service
├── card         // card service
│
├── org          // org service
└── user         // user service
```

Each service consists of Go files that implement the service business logic within a single `package`, database
migrations for defining the structure of the database(s), and so on. You can break your application in packages however
you like.

## Dividing your application in systems
Systems are a way to divide your application in subfolders for development purposes. Systems are not a special construct
in Encore, they only help you divide your application logically around common concerns and purposes. Encore only handles
services, the compiler will read your systems and extract the services of your application. As applications grow,
systems help you decompose your application without requiring any complex refactoring.

Going back to our Trello example, the app might consist of three systems: the **Trello** system (for managing trello
boards & cards), the **User** system (for user and organization management, and authentication), and the **Premium**
system (for subscriptions and paid features).

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

The only refactoring needed is to move services into their respective subfolders. This greatly simplifies the
application around the specific concerns of each system. For encore, what matters are the packages containing services.
The division in systems or subsystems will not change the endpoint and architecture of your application
