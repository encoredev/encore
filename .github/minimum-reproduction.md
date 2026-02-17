# Minimum Reproduction Repository

A minimum reproduction repository is a git repository that can be shared publicly (doesn't expose private business logic), shows the problem you're running into, and has the fewest dependencies installed possible. It also has the steps in place for how to replicate the error you're running into. This is easiest to add to the README.

## Doesn't Expose Business Logic

If your error resolves around a specific step in business logic, replicate the business logic in a way that doesn't make it evident what you're working on.

## Shows The Problem You're Running Into

This is why the minimum reproduction should be created in the first place, cause you have an error you want someone to look into.

## Has The Fewest Dependencies Installed Possible

If the reproduction doesn't need it, get rid of it.

## Steps To Replicate

A set of clear, defined steps on how to replicate the error. You can separate the setup and reproduction steps as well if you'd like. An example would be something like

```
# Setup

1) npm install

# Reproduction

1) encore run
2) open new terminal
3) curl http://localhost:4000/users
4) see the error
```

## Okay I Understand What It Is, What Else Do I Need?

Generally speaking, if you meet the above, it's good to go. This helps out those who debug errors and provide support immensely.

## So why am I being asked for this?

There's a few reasons to provide a minimum reproduction:

1. it makes debugging where the error _could_ be so much easier. Instead of looking across 20 files and 5 directories, it's now 2 files in 1 directory. Much less to dig through and understand
2. half the time while creating the minimum reproduction, you'll find what the problem was yourself and grow as a developer and as a knowledge sharer.