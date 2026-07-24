// Package ecl implements the Encore Configuration Language (ECL),
// a small declarative language for defining constraints and defaults
// over infrastructure resources.
//
// An ECL file contains resource blocks. A named block configures one
// resource of a kind, optionally scoped by a selector:
//
//	service "api" if env.type == "production" {
//	    cpu: >= 1 & <= 4 | default 2
//	    memory: >= 1Gi & <= 8Gi | default 1Gi
//	}
//
// A for block configures all resources of a kind matching the selector:
//
//	for service if env.type == "production" {
//	    cpu: >= 1 & <= 4 | default 1
//	}
//
// For a given resource, all matching blocks apply: constraints merge by
// intersection, while defaults resolve to the most specific matching
// block (one is more specific than another if its selector logically
// implies the other's; a named block is equivalent to a for block with a
// name == "..." selector). Exact value constraints (e.g.
// "public_access: false") also act as defaults when the property is unset.
//
// Environment-scoped rules can be grouped with if blocks, which desugar to
// ordinary selectors composed with &&. An if block tests only environment
// attributes (env.type, provider, ...; see RuleSet.EnvScope); per-resource
// conditions go on a rule's own if clause:
//
//	if env.type == production {
//	    for service { cpu: >= 1 & <= 4 | default 1 }
//	    for bucket { public_access: false }
//	}
//
// Whether a named block instantiates a managed resource (such as a
// sql_cluster) or only configures an app-discovered one (such as a
// service) is decided by the resource kind's schema (see Kind and
// DefaultSchema).
//
// Reference-valued properties point at another resource, either statically
// (kind.name) or dynamically (kind[expr]). A reference must resolve to an
// instantiated resource. Nested object syntax constrains the resolved
// target:
//
//	sql_database "audit" {
//	    cluster: sql_cluster.audit & {
//	        backup_retention: >= 90d
//	    }
//	}
//
// A dynamic block nested in a for block instantiates and configures
// kind/normalize(expr) per matching resource, merging resources that share
// a normalized name:
//
//	for service if tags.domain exists {
//	    instance: default service_instance[tags.domain]
//	    service_instance tags.domain {
//	        cpu: >= 1 & <= 8 | default 2
//	    }
//	}
//
// Use Definitions to enumerate the managed resources instantiated for an
// environment, and EvaluateEnv to evaluate a whole environment together,
// including instantiating managed and dynamic blocks and checking
// references.
//
// # Usage
//
// Parse one or more files with ParseFile (or Load to follow imports),
// combine them into a RuleSet, optionally run static analysis with
// Validate, and evaluate resources with Evaluate:
//
//	file, err := ecl.ParseFile("policies/services.encore", src)
//	rs := ecl.NewRuleSet(file)
//	if err := rs.Validate(); err != nil { ... }
//	result, err := rs.Evaluate(&ecl.Resource{
//	    Kind: "service",
//	    Name: "api",
//	    Attrs: map[string]ecl.Value{
//	        "env.type": ecl.String("production"),
//	    },
//	    Config: map[string]ecl.Value{
//	        "cpu": ecl.Number(2),
//	    },
//	})
//
// All errors returned by this package are of type ErrorList, a list of
// *Diagnostic values carrying precise source positions, source snippets,
// related notes, and remediation hints.
package ecl
