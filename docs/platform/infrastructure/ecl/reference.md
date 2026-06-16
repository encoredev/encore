---
seotitle: Encore Configuration Language (ECL) Reference
seodesc: Complete reference for the Encore Configuration Language (ECL) — syntax, types, selectors, constraints, defaults, references, and evaluation semantics.
title: ECL Language Reference
subtitle: Complete syntax and semantics of the Encore Configuration Language
lang: platform
---

This is the complete reference for the Encore Configuration Language (ECL). For a guided introduction, see the [Overview & Tutorial](/docs/platform/infrastructure/ecl).

## File structure

An ECL file consists of an optional version declaration, followed by imports, followed by declarations:

```
version 1

import "policies/common.encore"

for service {
    cpu: >= 0.25 & <= 8 | default 0.5
}
```

- **`version`** must be the first statement if present. The current language version is `1`.
- **`import "path"`** includes another file's blocks as if they were part of the same policy set. Imports are static — no parameters, no conditions — and must appear before the first block. Paths resolve relative to the importing file's directory first, then relative to the policy root. Importing the same file twice (including via cycles) includes it once.
- **Declarations** are resource blocks (`for`, named, and dynamic) and `if` blocks, in any order.

Statements are terminated by newlines. There are no semicolons. Lists inside `[...]` may span multiple lines.

### Comments

```
// Line comment.

/*
  Block comment.
*/
```

Comments have no semantic meaning.

### Identifiers and keywords

Identifiers start with a letter or underscore, followed by letters, digits, or underscores. The reserved keywords are:

```
for  if  in  exists  required  default  import  true  false
```

`version` is *not* reserved: the version declaration is recognized by position, so `version` remains usable as a property name (e.g. `version: "16"` in a database cluster).

## Resource blocks

A resource block targets a resource kind and contains property rules. It takes one of three forms:

```
for <kind> [ if <selector> ] { ... }       // every resource of the kind
<kind> "<name>" [ if <selector> ] { ... }  // one named resource
<kind> <expr> [ if <selector> ] { ... }    // dynamic: one resource per match
```

There are no priorities; a block's identity for diagnostics is its source location and its header.

A **named block** is shorthand for a `for` block with a `name` condition. These are identical:

```
service "api" if env.type == "production" { ... }
for service if name == "api" && env.type == "production" { ... }
```

A block applies to a resource when the kind matches, the name matches (if specified), and every selector condition matches.

Literal names must be quoted (`service "api"`). A bare expression after the kind (`service_instance tags.domain`) makes it a [dynamic block](#dynamic-blocks).

### Resource kinds

The built-in resource kinds are:

| Kind | Description | Managed |
|------|-------------|---------|
| `service` | Application services | no |
| `service_instance` | Service instances (managed) | yes |
| `bucket` | Object storage buckets | no |
| `pubsub_topic` | Pub/Sub topics | no |
| `sql_database` | Logical SQL databases | no |
| `sql_cluster` | SQL clusters (physical infrastructure) | yes |
| `cache` | Caches | no |
| `secret` | Secrets | no |
| `cron_job` | Cron jobs | no |

A block of an **app-discovered** kind (Managed *no*) only configures a resource discovered from your application code. A named or dynamic block of a **managed** kind (Managed *yes*) also *instantiates* the resource — it ensures the resource exists in matching environments. A `for` block never instantiates a resource; it only configures matching ones.

Each block targets exactly one kind. To apply the same rule to several kinds, repeat it per kind — similarly named properties may have different semantics across kinds, so the repetition is deliberate.

## Managed resources

A managed kind's named block both declares existence and configures the resource:

```
sql_cluster "main" if env.type == "production" {
    engine: "postgres"
    version: "16"
    cpu: >= 2 & <= 16 | default 4
}
```

This:

- **declares existence** — the named resource exists in every environment matching the selector;
- **participates in normal merging** — apart from declaring existence, it behaves exactly like a `for` block restricted to that name. Other blocks for the same resource merge with it.

Several blocks may target the same managed resource (for example with different selectors); the resource is created once and all matching blocks merge.

## `if` blocks

An `if` block groups declarations that apply only in a given environment:

```
if env.type == "production" {
    for service {
        cpu: >= 1 & <= 4 | default 1
    }
    sql_cluster "main" {
        engine: "postgres"
    }
}
```

This desugars to ordinary blocks with the conditions prepended to each selector:

```
for service if env.type == "production" { ... }
sql_cluster "main" if env.type == "production" { ... }
```

An `if` block is evaluated in the top-level **environment scope**, so its conditions may only reference environment attributes — by default any `env.*` field (`env.type`, `env.name`, …) and `provider`. Per-resource attributes (`name`, `team`, `tags.*`) are not in scope at the top level; testing one in an `if` block is an error. Put resource conditions on the rule's own `if` clause instead — the two compose with `&&`. The environment scope is configurable (`RuleSet.EnvScope`).

`if` blocks may contain resource blocks and nested `if` blocks. Nested blocks compose with `&&`: a rule inside `if env.type == "production" { for service if team == "x" { ... } }` has the effective selector `env.type == "production" && team == "x"`.

Because desugaring happens before evaluation, blocks in `if` blocks behave identically to their expanded form for matching, merging, and specificity.

## Selectors

A selector is one or more conditions combined with `&&`. All conditions must match.

| Operator | Example | Matches when |
|----------|---------|--------------|
| `==` | `env.type == "production"` | the field exists and equals the value |
| `!=` | `env.type != "preview"` | the field exists and differs from the value |
| `in` | `env.type in ["production", "staging"]` | the field exists and equals one of the listed values |
| `exists` | `tags.data exists` | the field is present, with any value |

There is no `||`. To express alternatives, use `in` for value alternatives, or split into separate blocks. Conditions on a missing field do not match (only `exists` distinguishes presence); `!=` on a missing field is *not* a match.

`in` is semantically equivalent to writing one block per value. For specificity purposes, `field == "v"` is more specific than `field in [..., "v", ...]`, and a smaller set is more specific than a superset.

### Selector fields

Fields are dot-separated paths. Common fields:

```
name            the resource's name (also settable via the block header)
env.name        environment name, e.g. "prod-eu"
env.type        environment type, e.g. production, staging, preview, development
team            owning team
business_unit   owning business unit
tags.<key>      resource tags
provider        cloud provider, e.g. gcp, aws
implementation  compute implementation, e.g. cloud_run, fargate
```

### String values

String values must be quoted; a bare identifier in value position is an error:

```
env.type == "production"      // ok
engine: "postgres"            // ok
env.type == production        // error: string values must be quoted
```

Numbers, booleans, sizes, and durations are written bare (`4`, `true`, `512Mi`, `30d`). A bare identifier is only valid as a kind, field, property path, or reference name — never as a value.

## Property rules

Inside a block body, each line defines a property rule:

```
<property-path>: <constraint>
<property-path>: default <value>
<property-path>: <constraint> | default <value>
```

Property paths are dot-separated, e.g. `cpu`, `instances.min`, `provider.gcp.cloud_run.min_instances`. A property may appear at most once per block; combine multiple constraints with `&`.

### Operator precedence

From highest to lowest:

1. comparisons and literals (`>= 1`, `false`, `"x"`)
2. `&` (conjunction)
3. `|` (disjunction)
4. `default` (lowest; only allowed as the final clause)

So `cpu: >= 1 & <= 4 | default 2` parses as *constraint `>= 1 & <= 4`, default `2`* — not as a disjunction containing the default. Writing `default` anywhere but at the end is an error:

```
cpu: >= 1 & <= 4 | default 2    // valid
cpu: default 2                  // valid
cpu: default 2 | <= 4           // error: 'default' must be the last clause
```

## Constraints

### Comparisons

```
cpu: >= 1 & <= 4
instances.max: <= 20
backup_retention: >= 30d
region: != "us-central1"
```

Ordering comparisons (`>=`, `<=`, `>`, `<`) apply to numbers, sizes, and durations. `==` and `!=` apply to all types.

### Exact values

A bare value is an exact constraint:

```
public_access: false
region: "europe-west1"
tier: == "small"            // explicit form, same meaning
```

An exact value constraint **also acts as a default**: if the property is unset, it is set to that value. `public_access: false` therefore means *must be false; defaults to false*. This implicit default participates in default resolution exactly like an explicit `default` clause.

No implicit default is inferred from ranges, inequalities, or disjunctions — only from a single exact value (optionally combined with `required`).

### Disjunctions

`|` lists allowed alternatives; the value must satisfy at least one:

```
region: "europe-west1" | "europe-north1"
tier: "small" | "medium" | "large"
```

Disjunctions from different matching blocks merge by intersection: if one block allows `a | b | c` and another allows `a | b`, the effective constraint is `a | b`.

### Conjunctions

`&` combines constraints; all must hold:

```
cpu: >= 1 & <= 4
backup_retention: required & >= 30d
```

Note that selectors use `&&` while constraints use `&`.

### `required`

`required` demands that the final resolved configuration contains the property:

```
backup_retention: required & >= 30d
```

This is satisfied if the resource sets the property explicitly, or if any matching block provides a default for it. `required` may only appear as a top-level conjunct — it cannot be an alternative in a `|` disjunction.

## Values and types

Every scalar value has one of five types. Constraints, defaults, and resource configuration for a property must all use the same type; mixing types is an error.

| Type | Examples | Notes |
|------|----------|-------|
| number | `1`, `0.5`, `-2` | Decimal notation; no scientific notation |
| bool | `true`, `false` | |
| string | `"europe-west1"`, `"production"` | Must be quoted; escapes `\"` `\\` `\n` `\t` `\r` |
| size | `512Mi`, `8Gi`, `100GB` | A number with a size unit |
| duration | `30s`, `12h`, `30d` | A number with a duration unit |

Reference-valued properties hold a [reference](#references) rather than a scalar.

### Size units

| Unit | Meaning | | Unit | Meaning |
|------|---------|-|------|---------|
| `B` | bytes | | `KB` | 10³ bytes |
| `Ki` | 2¹⁰ bytes | | `MB` | 10⁶ bytes |
| `Mi` | 2²⁰ bytes | | `GB` | 10⁹ bytes |
| `Gi` | 2³⁰ bytes | | `TB` | 10¹² bytes |
| `Ti` | 2⁴⁰ bytes | | | |

### Duration units

| Unit | Meaning |
|------|---------|
| `ms` | milliseconds |
| `s` | seconds |
| `m` | minutes |
| `h` | hours |
| `d` | days |

Sizes and durations compare by canonical value, so `1Gi == 1024Mi` and `1m == 60s`. Sizes and numbers are distinct types: `1024` (number) does not equal `1024B` (size).

## References

Some properties point at another resource. A reference is written as a value of that property:

```
cluster: sql_cluster.main                  // static reference
cluster: sql_cluster[tags.domain]          // dynamic reference
cluster: default sql_cluster.main          // reference as a default
```

- A **static reference** `kind.name` names a specific resource.
- A **dynamic reference** `kind[expr]` evaluates the attribute path `expr` against the resource and uses the [normalized](#name-normalization) result as the name.

Which properties are reference-valued, and the kind they refer to, is fixed by the resource kind's schema — for example a `sql_database`'s `cluster` refers to a `sql_cluster`, and a `service`'s `instance` refers to a `service_instance`.

A bare reference (`cluster: sql_cluster.main`) is an exact value: it is both an identity constraint and an implicit default, exactly like a bare scalar value. Use `default` for a reference that can be overridden.

References never create resources. A reference **must resolve to an instantiated resource** in the environment; if it does not, evaluation fails.

### Object constraints

Reference-valued properties can carry nested constraints on the resolved target, in braces:

```
for sql_database if tags.data == "customer" {
    cluster: {
        backup_retention: >= 30d
        point_in_time_recovery: true
        high_availability: true
    }
}
```

This does not choose a target — the identity comes from the reference elsewhere. It constrains *whichever* resource the property resolves to. Every property listed must be present on the target's resolved configuration and satisfy its constraint; an absent property is an error. Object constraints from all matching blocks apply, like ordinary constraints. `default` is not allowed inside an object constraint, since it constrains another resource.

Identity and object constraints can be combined with `&`:

```
sql_database "audit" {
    cluster: sql_cluster.audit & {
        backup_retention: >= 90d
    }
}
```

## Dynamic blocks

A dynamic block nested in a `for` block instantiates and configures a resource per matching resource of the enclosing block. The name comes from an attribute expression evaluated against the matching resource:

```
for service if tags.domain exists {
    instance: default service_instance[tags.domain]
    service_instance tags.domain {
        cpu: >= 1 & <= 8 | default 2
        memory: >= 1Gi & <= 16Gi | default 4Gi
    }
}
```

For each matching service, Encore evaluates `tags.domain`, normalizes it into a name, and instantiates/configures `service_instance.<name>`. Services whose expression normalizes to the same name contribute to the *same* resource, and their configurations merge.

### Name normalization

A dynamic block's or reference's evaluated value is normalized into a valid resource name:

- surrounding whitespace is trimmed;
- the value is lowercased;
- each run of invalid characters (anything other than `a`–`z`, `0`–`9`, or `-`) becomes a single `-`;
- leading and trailing `-` are removed;
- an empty result is rejected.

If two distinct source values normalize to the same name (for example `"Billing API"` and `"billing-api"` both yield `billing-api`), that is a collision and evaluation fails.

## Evaluation semantics

An environment is evaluated as a whole:

1. Instantiate managed resources: named blocks of managed kinds, and dynamic blocks fired against the input resources.
2. For each resource, find all matching blocks (kind, name, selector).
3. Merge all matching constraints.
4. Resolve each scalar property's default from the most specific matching block; resolve each reference property's target.
5. Apply defaults to unset properties.
6. Validate the final configuration, check that every reference resolves to an existing resource, and check object constraints against the referenced resources.

### Constraint merging

Constraints are cumulative across all matching blocks — they merge by **intersection**. A matching block can narrow another block's constraints but never weaken them:

```
for service {
    cpu: >= 0.25 & <= 8
}
for service if env.type == "production" {
    cpu: >= 1 & <= 4
}
for service if team == "payments" {
    cpu: >= 2 & <= 6
}
```

For a production payments service the effective constraint is `cpu: >= 2 & <= 4`: the highest minimum and the lowest maximum.

If the merged constraints admit no possible value — conflicting exact values, an empty range, an empty intersection of allowed values — evaluation fails even if the property is unset.

### Default resolution and specificity

Defaults need a single winner. The rule is:

> The default comes from the most specific matching block. If no matching block is most specific, the default is ambiguous and evaluation fails.

A block is **more specific** than another when its selector *logically implies* the other's: every resource matching the first necessarily matches the second. A named block's name participates as a `name == "..."` condition. For example:

```
for service                                                   // least specific
for service if env.type == "production"                      // implies the above
for service if env.type == "production" && team == "payments"
service "api" if env.type == "production" && team == "payments"  // most specific
```

Each block in this chain implies all blocks above it, so for a matching resource the last block's default wins.

Implication is structural, not positional: `team == "payments"` implies `team in ["payments", "billing"]`, and `env.type == "production"` implies `env.type exists`. Source order never matters.

Two matching blocks with different defaults where neither implies the other is an **error** — for example `env.type == "production"` versus `team == "payments"` for a resource matching both. The fix is a block covering the combined selector, which then strictly implies both and wins. If multiple matching blocks provide the *same* default value, there is no conflict.

The same specificity rule decides reference identity: when several blocks point a reference at different targets, the most specific wins, and a tie is an error.

### Explicit values vs. defaults

Explicit resource configuration always beats defaults, but never beats constraints:

```
for service if env.type == "production" {
    cpu: >= 1 & <= 4 | default 1
}
```

| Resource config | Result |
|-----------------|--------|
| `cpu = 3` | Accepted: explicit value satisfies the constraint |
| `cpu = 8` | Rejected: violates `<= 4` |
| unset | `cpu` defaults to 1 |

A default must itself satisfy all matching constraints — `cpu: <= 4 | default 8` is rejected, as is a default in one block that violates a constraint from another matching block.

### Required properties

`required` is satisfied by an explicit value or by an applicable default:

```
for sql_database if env.type == "production" {
    backup_retention: required & >= 30d
}
for sql_database if env.type == "production" {
    backup_retention: default 30d
}
```

The result: `backup_retention` is always present in production, defaulting to `30d`.

## Validation

Beyond per-resource evaluation, a policy set is checked statically for problems that don't depend on any particular resource:

- unknown resource kinds (with did-you-mean suggestions)
- reference properties pointed at the wrong kind, scalar constraints on a reference property, or a reference on a non-reference property
- selectors that can never match (e.g. `env.type == "production" && env.type == "staging"`)
- duplicate property rules within a block or object constraint
- mixed value types within a property rule (e.g. `cpu: >= 1 & <= 2Gi`)
- defaults that violate their own block's constraint
- pairs of blocks that can match the same resource but produce impossible merged constraints
- defaults that violate the constraints of another block that can match the same resource

Ambiguous defaults and unresolved references are *not* rejected statically, because they depend on which resources exist in a concrete environment; they are reported during evaluation.

All errors include the source location, a snippet of the offending line, the blocks involved, and — where possible — a suggested fix.

## Grammar

```
file        = [ version ] { import } { decl } .
version     = "version" number .
import      = "import" string .

decl        = for-block | named-block | dynamic-block | if-block .
for-block     = "for" ident [ "if" selector ] "{" body "}" .
named-block   = ident string [ "if" selector ] "{" body "}" .
dynamic-block = ident field [ "if" selector ] "{" body "}" .
if-block      = "if" selector "{" { decl } "}" .

body        = { property | named-block | dynamic-block } .
property    = field ":" prop-expr .
prop-expr   = "default" ( value | reference )
            | or-expr [ "|" "default" ( value | reference ) ] .
or-expr     = and-expr { "|" and-expr } .
and-expr    = term { "&" term } .
term        = "required"
            | comp-op value
            | reference
            | object
            | value .
object      = "{" { property } "}" .
comp-op     = "==" | "!=" | ">=" | "<=" | ">" | "<" .

reference   = ident "." ident             // static: kind.name
            | ident "[" field "]" .        // dynamic: kind[expr]

selector    = condition { "&&" condition } .
condition   = field "exists"
            | field ( "==" | "!=" ) value
            | field "in" "[" value { "," value } [ "," ] "]" .

field       = ident { "." ident } .
value       = [ "-" ] number [ unit ]
            | string
            | "true" | "false" .
```

Property rules, imports, and declarations are terminated by newlines; lists inside `[...]` may span lines.
