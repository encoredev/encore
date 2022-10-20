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
	hide_in_menu?: bool
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
			{title: "Tutorials", segment: "index", hide_in_menu: true},
			{title: "Building a REST API", segment: "rest-api"},
			{title: "Building a Slack bot", segment: "slack-bot"},
			{title: "Building an Incident Tool", segment: "incident-management-tool"},
		]
	},
	{
		title: "Develop"
		segment: "develop"
		docs: [
			{title: "App Structure", segment: "app-structure"},
			{title: "Services and APIs", segment: "services-and-apis"},
			{title: "API Schemas", segment: "api-schemas"},
			{title: "API Errors", segment: "errors", old_paths:["/concepts/errors"], shortcuts: ["beta/errs", "errs"]},
			{title: "Authentication", segment: "auth", shortcuts: ["beta/auth", "auth"]},
			{title: "SQL Databases", segment: "databases", old_paths:["/develop/sql-database"], shortcuts: ["storage/sqldb"]},
			{
				title: "Cron Jobs",
				segment: "cron-jobs"
				old_paths: [
					"/cron-jobs" // linked to from our go package
				],
				 shortcuts: ["cron"]
			},
			{title: "Configuration", segment: "config", shortcuts: ["config"]},
			{title: "Secrets", segment: "secrets"},
			{title: "PubSub", segment: "pubsub", shortcuts: ["pubsub"]},
			{title: "Caching", segment: "caching", shortcuts: ["storage/cache"]},
			{title: "Testing", segment: "testing"},
			{title: "Middleware", segment: "middleware"},
			{title: "Validation", segment: "validation"},
			{title: "Metadata", segment: "metadata"},
			{title: "Generated API Docs", segment: "api-docs"},
			{title: "Client Generation", segment: "client-generation"},
			{title: "CLI Reference", segment: "cli-reference"},
			{title: "Encore Flow", segment: "encore-flow"},
		]
	},
	{
		title: "Deploy"
		segment: "deploy"
		docs: [
			{title: "Cloud Infrastructure", segment: "infra", old_paths: ["/docs/deploy/scaling", "/docs/concepts/scaling"]},
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
			{title: "Logging", segment: "logging", shortcuts: ["rlog"]},
			{title: "Monitoring", segment: "monitoring"},
			{title: "Distributed Tracing", segment: "tracing"},
		]
	},
	{
		title: "Configuration"
		segment: "configuration"
		docs: [
			{title: "Auth Keys", segment: "auth-keys"},
		]
	},
	{
		title: "How-to Guides"
		segment: "how-to"
		docs: [
			{title: "Debug with Delve", segment: "debug"},
			{title: "Change SQL database schema", segment: "change-db-schema"},
			{title: "Connect to an existing database", segment: "connect-existing-db"},
			{title: "Share SQL databases between services", segment: "share-db-between-services"},
			{title: "Insert test data in a database", segment: "insert-test-data-db"},
			{title: "Receive webhooks", segment: "webhooks"},
			{title: "Use Dependency Injection", segment: "dependency-injection"},
			{title: "Integrate with a web frontend", segment: "integrate-frontend"},
			{title: "Use Firebase Authentication", segment: "firebase-auth"},
			{title: "Integrate with GitHub", segment: "github"},
			{title: "Migrate away from Encore", segment: "migrate-away"},
			{title: "Use the ent ORM for migrations", segment: "entgo-orm"},
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
