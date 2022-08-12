package docs

import "strings"

#Document: {
	title: string
	segment: string
	section: string
	category: string
	path: string | *strings.Replace("/\(section)/\(segment)", "/index", "", -1)
	old_paths: [string] | *null
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
			{title: "Overview", segment: "index"},
			{title: "Why Encore?", segment: "benefits"},
			{title: "Encore Application Model", segment: "application-model"},
			{title: "The Encore Way", segment: "encore-way", old_paths: ["/deploy/platform"]},
			{title: "Installation", segment: "install"},
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
			{title: "Building a REST API", segment: "rest-api"},
			{title: "Building a Slack bot", segment: "slack-bot"},
		]
	},
	{
		title: "Develop"
		segment: "develop"
		docs: [
			{title: "Services and APIs", segment: "services-and-apis"},
			{title: "API Errors", segment: "errors", old_paths:["/concepts/errors"]},
			{title: "Authentication", segment: "auth"},
			{title: "SQL Databases", segment: "databases"},
			{
				title: "Cron Jobs",
				segment: "cron-jobs"
				old_paths: [
					"/cron-jobs" // linked to from our go package
				]
			},
			{title: "Configuration", segment: "config"},
			{title: "Secrets", segment: "secrets"},
			{title: "Testing", segment: "testing"},
			{title: "Metadata", segment: "metadata"},
			{title: "Generated API Docs", segment: "api-docs"},
			{title: "CLI Reference", segment: "cli-reference"},
		]
	},
	{
		title: "Deploy"
		segment: "deploy"
		docs: [
			{title: "Scaling", segment: "scaling"},
			{title: "Environments", segment: "environments"},
			{title: "Bring your own cloud", segment: "own-cloud"},
			{title: "Custom Domains", segment: "custom-domains"},
			{title: "Infrastructure", segment: "infra"},
			{title: "Security", segment: "security"},
		]
	},
	{
		title: "Observability"
		segment: "observability"
		docs: [
			{title: "Development Dashboard", segment: "dev-dash"},
			{title: "Logging", segment: "logging"},
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
			{title: "Receive webhooks", segment: "webhooks"},
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
			source: old_path,
			destination: doc.path,
			permanent: true
		}
	}
]
