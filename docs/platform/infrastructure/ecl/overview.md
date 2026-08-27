---
seotitle: Encore Configuration Language (ECL) — Overview & Tutorial
seodesc: Learn how to define infrastructure constraints and defaults for your Encore applications using the Encore Configuration Language (ECL).
title: Configuration Language (ECL)
subtitle: Define infrastructure constraints and defaults as declarative policy
lang: platform
---

The Encore Configuration Language (ECL) is a small declarative language for defining rules over your infrastructure resources: what values are allowed, and what values apply by default.

Instead of configuring each resource in each environment by hand, you write rules like:

```
for service if env.type == "production" {
    cpu: >= 1 & <= 4 | default 1
    memory: >= 1Gi & <= 8Gi | default 1Gi
}
```

This single rule expresses both a *guardrail* (production services must have between 1 and 4 CPUs) and a *default* (if a service doesn't specify CPU, it gets 1). Explicit configuration always wins over defaults — but it can never violate a constraint.

ECL is designed to stay small and predictable:

- **There are no priorities.** A block is identified by what it targets, not by a name or a ranking. Diagnostics refer to the file and line where a block is defined.
- **Constraints merge by intersection.** When several blocks match the same resource, all of their constraints apply. A block can narrow what another allows, but never widen it.
- **Defaults resolve by specificity.** When several blocks provide defaults, the most specific one wins. If no block is most specific, it's an error — never a silent, arbitrary choice.

This page is a hands-on tutorial. For the complete syntax and semantics, see the [Language Reference](/docs/platform/infrastructure/ecl/reference).

## The three core ideas

Everything in ECL is built from three constructs:

1. **Named blocks** configure one specific resource: `service "api" { ... }`.
2. **`for` blocks** configure every resource of a kind matching a predicate: `for service if env.type == "production" { ... }`.
3. **References** point one resource at another: `kind.name` (static) or `kind[expr]` (dynamic).

## Your first block

An ECL file contains a list of blocks. A `for` block targets every resource of a kind:

```
// Baseline limits for all services, in all environments.
for service {
    cpu: >= 0.25 & <= 8 | default 0.5
    memory: >= 256Mi & <= 16Gi | default 512Mi
}
```

Reading the `cpu` line left to right:

- `>= 0.25 & <= 8` is the **constraint**: if `cpu` is set, it must be between 0.25 and 8. `&` combines constraints; all of them must hold.
- `| default 0.5` is the **default**: if `cpu` is not set, use 0.5.

Values can be plain numbers, booleans, strings, sizes (`512Mi`, `8Gi`), or durations (`30s`, `30d`).

## Narrowing rules with selectors

An `if` clause limits which resources a block applies to:

```
for service if env.type == "production" {
    cpu: >= 1 & <= 4 | default 1
}
```

Now consider a production service with both blocks in effect. Constraints **merge by intersection**: the service must satisfy `>= 0.25 & <= 8` *and* `>= 1 & <= 4`, which works out to `>= 1 & <= 4`. The effective minimum is the highest minimum, and the effective maximum is the lowest maximum.

Defaults work differently — only one can apply. The production block's selector is **more specific** (every production service is also a service, but not vice versa), so its default of 1 wins over the baseline 0.5.

Selectors support equality, inequality, membership, and existence checks, combined with `&&`:

```
for service if env.type == "production" && team == "payments" {
    cpu: default 2
}

for service if env.type in ["production", "staging"] {
    instances.min: >= 1 | default 1
}

for bucket if tags.data exists {
    backup_retention: >= 7d
}
```

String values must be quoted: write `env.type == "production"`, not `env.type == production`. Numbers, booleans, sizes, and durations are written bare.

## Configuring one specific resource

To configure one particular resource, use a **named block** — the kind followed by the name:

```
service "api" if env.type == "production" {
    cpu: default 3
}
```

A named block is shorthand for a `for` block with a `name` condition. These are identical:

```
service "api" if env.type == "production" { ... }
for service if name == "api" && env.type == "production" { ... }
```

Because the named block's selector implies the unnamed production block's selector, it is more specific, and its default wins for the `api` service.

## Exact values

For boolean and enum-style settings, write the value directly:

```
for bucket {
    public_access: false
    versioning: true
}
```

An exact value is a constraint — `public_access` *must* be `false` — and it also acts as an implicit default: if the property is unset, it is set to `false`. This keeps common policy rules concise; you don't need to write `public_access: false | default false`.

Ranges do not imply defaults, because there's no single obvious value. `cpu: >= 1 & <= 4` constrains `cpu` but leaves it unset unless a `default` says otherwise.

## When rules conflict

ECL refuses to guess. Two situations are errors:

**Impossible constraints.** If merged constraints admit no possible value, evaluation fails — even before anyone sets the property:

```
for service if env.type == "production" {
    cpu: >= 4
}
for service if team == "payments" {
    cpu: <= 2
}
```

For a production payments service, no valid `cpu` exists, and you get an error pointing at both blocks:

```
policy.encore:5:10: error: impossible constraints for property 'cpu' of service "api": '>= 4' conflicts with '<= 2': no value can satisfy both
   |
 5 |     cpu: <= 2
   |          ^^^^
  '>= 4' at policy.encore:2:10 in rule: for service if env.type == "production"
  '<= 2' at policy.encore:5:10 in rule: for service if team == "payments"
  help: constraints from all matching rules merge by intersection; a rule cannot weaken another rule's constraints
```

**Ambiguous defaults.** If two matching blocks provide *different* defaults and neither selector implies the other, there is no most-specific block:

```
for service if env.type == "production" {
    cpu: default 1
}
for service if team == "payments" {
    cpu: default 2
}
```

A production payments service matches both, and neither block is more specific:

```
policy.encore:2:18: error: ambiguous default for property 'cpu' of service "api"
   |
 2 |     cpu: default 1
   |                  ^
  matching rules provide different defaults:
    policy.encore:1:1: for service if env.type == "production"
        cpu: default 1
    policy.encore:4:1: for service if team == "payments"
        cpu: default 2
  no rule is more specific than all the others
  help: add a more specific rule that decides the default, e.g.:
    for service if env.type == "production" && team == "payments" {
        cpu: default 2
    }
```

The fix is exactly what the error suggests: add a block whose selector covers both, which is then strictly more specific and breaks the tie. (If both blocks provide the *same* default value, there is no conflict.)

## Grouping rules by environment

When many blocks apply only in a certain environment, group them in an `if` block:

```
if env.type == "production" {
    for service {
        cpu: >= 1 & <= 4 | default 1
    }
    for bucket {
        public_access: false
    }
}
```

This is identical to writing `if env.type == "production"` on each block. An `if` block can only test *environment* attributes — the things in scope at the top level, like `env.type` or `provider`. Per-resource conditions (a team, a tag) belong on a rule's own `if` clause, and the two compose with `&&`:

```
if env.type == "production" {
    for service if team == "payments" {
        cpu: default 2
    }
}
```

The inner rule means `for service if env.type == "production" && team == "payments"`. Writing `if team == "payments"` is an error — `team` is a per-resource attribute, not an environment one, so it can't be tested at the top level.

Nested `if` blocks compose the same way.

## Managed infrastructure

Some resources are physical infrastructure that Encore creates and manages — like a SQL cluster — rather than logical resources discovered from your application code. You don't declare these with a special keyword: whether a named block *instantiates* a managed resource or just *configures* an app-discovered one is decided by the resource kind. `service` is app-discovered; `sql_cluster` is managed.

So a named block of a managed kind both creates the resource and configures it:

```
sql_cluster "main" if env.type == "production" {
    engine: "postgres"
    version: "16"
    cpu: >= 2 & <= 16 | default 4
    memory: >= 8Gi & <= 64Gi | default 16Gi
    storage: >= 100Gi | default 100Gi
    backup_retention: >= 30d | default 30d
    high_availability: true
}
```

In a matching environment this ensures a `sql_cluster` named `main` exists, with these constraints and defaults applied. Other blocks for the same resource merge with it as usual.

## References

A logical database remains an app-level resource. Its `cluster` property is a **reference** to a `sql_cluster` — it points the database at a cluster, but never creates one:

```
// Production databases run on the main cluster unless overridden.
for sql_database if env.type == "production" {
    cluster: default sql_cluster.main
}

// The audit database is pinned to the audit cluster.
sql_database "audit" if env.type == "production" {
    cluster: sql_cluster.audit
}
```

`kind.name` is a **static reference**. Note the difference: `cluster: default sql_cluster.main` means *use main unless explicitly overridden*, while `cluster: sql_cluster.audit` is an exact value — the audit database *must* use the audit cluster.

A reference must resolve to a resource that actually exists in the environment. If `sql_cluster.main` is never instantiated by a matching block, the reference is an error.

## Constraining the referenced resource

Some database requirements are really requirements on the *cluster the database ends up on*. Express those with nested object syntax on the reference property:

```
for sql_database if env.type == "production" && tags.data == "customer" {
    cluster: {
        backup_retention: >= 30d
        point_in_time_recovery: true
        high_availability: true
    }
}
```

This does not choose a cluster — the identity still comes from the `cluster` reference elsewhere. It says: *whichever cluster this database is placed on must satisfy these constraints.* Every property listed must be present on the selected cluster and satisfy its constraint; if the cluster doesn't set `point_in_time_recovery` at all, that's an error.

You can combine an identity and constraints in one property:

```
sql_database "audit" {
    cluster: sql_cluster.audit & {
        backup_retention: >= 90d
    }
}
```

## Dynamic resources

Sometimes the resource to create — or to point at — isn't known until you look at each matching resource. A **dynamic reference** uses bracket syntax with an attribute expression, and a **dynamic block** instantiates a resource named by that expression:

```
for service if tags.domain exists {
    instance: default service_instance[tags.domain]
    service_instance tags.domain {
        cpu: >= 1 & <= 8 | default 2
        memory: >= 1Gi & <= 16Gi | default 4Gi
    }
}
```

For each service tagged with a domain, this instantiates a `service_instance` named after the domain, and points the service's `instance` at it. Services sharing the same `tags.domain` contribute to the *same* `service_instance`, and their configurations merge.

The evaluated expression is normalized into a valid resource name: whitespace is trimmed, the value is lowercased, invalid characters become `-`, and repeated dashes collapse. If two different source values normalize to the same name (for example `"Billing API"` and `"billing-api"`), that's an error.

## Putting it all together

A realistic production policy file:

```
version 1

if env.type == "production" {
    // Baseline production services.
    for service {
        cpu: >= 1 & <= 4 | default 1
        memory: >= 1Gi & <= 8Gi | default 1Gi
        instances.min: >= 1 | default 1
    }

    // Specific service override.
    service "api" {
        cpu: default 2
    }

    // Shared default production SQL cluster.
    sql_cluster "main" {
        engine: "postgres"
        version: "16"
        cpu: >= 2 & <= 16 | default 4
        memory: >= 8Gi & <= 64Gi | default 16Gi
        storage: >= 100Gi | default 100Gi
        backup_retention: >= 30d | default 30d
        point_in_time_recovery: true
        high_availability: true
    }

    // Production databases use main unless otherwise configured.
    for sql_database {
        cluster: default sql_cluster.main
    }

    // Audit database gets a dedicated cluster.
    sql_database "audit" {
        cluster: sql_cluster.audit & {
            backup_retention: >= 90d
        }
    }
    sql_cluster "audit" {
        engine: "postgres"
        version: "16"
        cpu: >= 4 & <= 32 | default 8
        memory: >= 16Gi & <= 128Gi | default 32Gi
        storage: >= 500Gi | default 1Ti
        backup_retention: >= 90d | default 90d
    }

    // Customer-data databases constrain whichever cluster they use.
    for sql_database if tags.data == "customer" {
        cluster: {
            backup_retention: >= 30d
            point_in_time_recovery: true
            high_availability: true
        }
    }

    for bucket {
        public_access: false
        versioning: true
    }
}
```

In a production environment, this instantiates two SQL clusters, points every database at `main` by default while pinning the audit database to its own cluster, enforces guardrails on services, and keeps buckets private and versioned. Customer-data databases additionally require their cluster — whichever one they end up on — to meet the data-protection bar.

As files grow, split them up and use imports:

```
import "policies/services.encore"
import "policies/storage.encore"
```

Imported files are evaluated as if their blocks were part of the same policy set.

## How evaluation works

Encore evaluates a whole environment together:

1. Instantiate managed resources: named blocks of managed kinds, plus any dynamic blocks fired by matching resources.
2. For each resource, find all matching blocks (by kind, name, and selector).
3. Merge all matching constraints by intersection.
4. Resolve each property's default from the most specific matching block, and resolve reference-valued properties to their target.
5. Apply defaults to unset properties. Explicit configuration wins over defaults.
6. Validate the final configuration against every constraint, check that every reference resolves to an existing resource, and check object constraints against the referenced resources.

If anything is wrong — a violated constraint, an impossible combination, an ambiguous default, an unresolved reference — evaluation fails with an error pointing at the exact blocks and source lines involved.

## Next steps

The [Language Reference](/docs/platform/infrastructure/ecl/reference) covers the complete syntax and semantics: all operators and literal types, references and dynamic blocks, the precise specificity and merging rules, `required` properties, and the full grammar.
