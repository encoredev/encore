package docs

import "strings"

#Document: {
	title: string
	segment: string
	section: string
	category: string
	path: string | *strings.Replace("/\(section)/\(segment)", "/index", "", -1)
	old_paths: [...string] | *null
	shortcuts: [...string] | *null // URL's which can be used on the root of the website or on docs if it would have been a 404 (i.e. https://encore.dev/topics => https://encore.dev/docs/develop/pubsub)
}

#Section: {
	title: string
	segment: string
	docs: [...#Document & {
		category: title
		section: segment
	}]
}

#Redirect: {
	source: string
	destination: string
	permanent: bool | *true
}

sections: [...#Section]
sections: [
	{
		title: "Introduction"
		segment: "index"
		docs: [
			{title: "Welcome", segment: "index"},
			{title: "Installation", segment: "install"},
			{
				title: "What is Encore?",
				segment: "introduction",
				old_paths: ["/benefits", "/application-model", "/encore-way"]
			},
			{
				title: "Quick Start",
				segment: "quick-start",
				old_paths: [ "/tutorials/create-app"]
			},
		]
	},
	{
		title: "Tutorials"
		segment: "tutorials"
		docs: [
			{title: "Overview", segment: "index"},
			{title: "Building an Uptime Monitor", segment: "uptime"},
			{title: "Building a REST API", segment: "rest-api"},
			{title: "Building a Slack bot", segment: "slack-bot"},
			{title: "Building an Incident Tool", segment: "incident-management-tool"},
		]
	},
	{
		title: "Building blocks"
		segment: "primitives"
		docs: [
			{title: "Overview", segment: "overview"},
			{title: "Services and APIs", segment: "services-and-apis", old_paths:["/develop/services-and-apis"]},
			{title: "Databases", segment: "databases", old_paths:["/develop/databases","/develop/sql-database"], shortcuts: ["storage/sqldb"]},
			{title: "Cron Jobs", segment: "cron-jobs", old_paths:["/cron-jobs","/develop/cron-jobs"], shortcuts: ["cron"]},
			{title: "PubSub", segment: "pubsub", old_paths:["/develop/pubsub"], shortcuts: ["pubsub"]},
			{title: "Caching", segment: "caching", old_paths:["/develop/caching"], shortcuts: ["storage/cache"]},
			{title: "Secrets", segment: "secrets", old_paths:["/develop/secrets"]},
			{title: "Code Snippets", segment: "code-snippets"},
		]
	},
	{
		title: "Develop"
		segment: "develop"
		docs: [
			{title: "App Structure", segment: "app-structure"},
			{title: "API Schemas", segment: "api-schemas"},
			{title: "API Errors", segment: "errors", old_paths:["/concepts/errors"], shortcuts: ["beta/errs", "errs"]},
			{title: "Authentication", segment: "auth", shortcuts: ["beta/auth", "auth"]},
			{title: "Configuration", segment: "config", shortcuts: ["config"]},
			{title: "Metadata", segment: "metadata"},
			{title: "Auth Keys", segment: "auth-keys", old_paths:["/configuration/auth-keys"]},
			{title: "Testing", segment: "testing"},
			{title: "Middleware", segment: "middleware"},
			{title: "Validation", segment: "validation"},
			{title: "Generated API Docs", segment: "api-docs"},
			{title: "Client Generation", segment: "client-generation"},
			{title: "CLI Reference", segment: "cli-reference"},
		]
	},
	{
		title: "Deploy"
		segment: "deploy"
		docs: [
			{title: "Infrastructure provisioning", segment: "infra", old_paths: ["/deploy/scaling", "/concepts/scaling"]},
			{title: "Environments", segment: "environments"},
			{title: "Connect your cloud account", segment: "own-cloud"},
			{title: "Custom Domains", segment: "custom-domains"},
			{title: "Security", segment: "security"},
		]
	},
	{
		title: "Observability"
		segment: "observability"
		docs: [
			{title: "Development Dashboard", segment: "dev-dash"},
			{title: "Flow Architecture Diagram", segment: "encore-flow", old_paths: ["/develop/encore-flow"]},
			{title: "Logging", segment: "logging", shortcuts: ["rlog"]},
			{title: "Metrics", segment: "metrics", shortcuts: ["metrics"], old_paths: ["/observability/monitoring"]},
			{title: "Distributed Tracing", segment: "tracing"},
		]
	},
	{
		title: "How-to Guides"
		segment: "how-to"
		docs: [
			{title: "Build with cgo", segment: "cgo"},
			{title: "Debug with Delve", segment: "debug"},
			{title: "Change SQL database schema", segment: "change-db-schema"},
			{title: "Connect to an existing database", segment: "connect-existing-db"},
			{title: "Share SQL databases between services", segment: "share-db-between-services"},
			{title: "Insert test data in a database", segment: "insert-test-data-db"},
			{title: "Use Temporal", segment: "temporal"},
			{title: "Use the ent ORM for migrations", segment: "entgo-orm"},
			{title: "Receive webhooks", segment: "webhooks"},
			{title: "Use Dependency Injection", segment: "dependency-injection"},
			{title: "Integrate with a web frontend", segment: "integrate-frontend"},
			{title: "Use Firebase Authentication", segment: "firebase-auth"},
			{title: "Integrate with GitHub", segment: "github"},
			{title: "Migrate an existing backend to Encore", segment: "migrate-to-encore"},
			{title: "Migrate away from Encore", segment: "migrate-away"},
		]
	},
	{
		title: "Other Topics"
		segment: "other"
		docs: [
			{title: "Encore vs. Terraform / Pulumi", segment: "vs-terraform"},
			{title: "Encore vs. Heroku", segment: "vs-heroku"},
			{title: "Encore vs. Supabase / Firebase", segment: "vs-supabase"},
		]
	},
	{
		title: "Community"
		segment: "community"
		docs: [
			{title: "Get involved", segment: "index"},
			{title: "Contribute", segment: "contribute"},
			{title: "Principles", segment: "principles"},
			{title: "Open Source", segment: "open-source"},
		]
	},
	{
		title: "About"
		segment: "about"
		docs: [
			{title: "Roles & Permissions", segment: "permissions"},
			{title: "Usage limits", segment: "usage"},
			{title: "Plans & billing", segment: "billing"},
		]
	},
]

flattened: [...#Document]
flattened: [
	for sec in sections
	for doc in sec.docs { doc }
]

redirects: [...#Redirect]
redirects: [
	for sec in sections
	for doc in sec.docs
	if doc.old_paths != null
	for old_path in doc.old_paths {
		{
			source: "/docs\(old_path)",
			destination: "/docs\(doc.path)",
			permanent: true
		}
	}
]


shortcuts: [...#Redirect]
shortcuts: [
  for sec in sections
	for doc in sec.docs
	if doc.shortcuts != null
	for shortcut in doc.shortcuts {
		{
			source: shortcut,
			destination: "/docs\(doc.path)",
			permanent: false
		}
	}
]
