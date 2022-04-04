package docs

import "strings"

#Document: {
	title: string
	segment: string
	section: string
	category: string
	path: string | *strings.Replace("/\(section)/\(segment)", "/index", "", -1)
}

#Section: {
	title: string
	segment: string
	docs: [...#Document & {
		category: title
		section: segment
	}]
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
			{title: "Quick Start", segment: "quick-start"},
		]
	},
	{
		title: "Tutorials"
		segment: "tutorials"
		docs: [
			{title: "Creating your first app", segment: "create-app"},
			{title: "Building a REST API", segment: "rest-api"},
			{title: "Building a Slack bot", segment: "slack-bot"},
		]
	},
	{
		title: "Develop"
		segment: "develop"
		docs: [
			{title: "Services and APIs", segment: "services-and-apis"},
			{title: "API Errors", segment: "errors"},
			{title: "Authentication", segment: "auth"},
			{title: "SQL Databases", segment: "databases"},
			{title: "Cron Jobs", segment: "cron-jobs"},
			{title: "Secrets", segment: "secrets"},
			{title: "Testing", segment: "testing"},
			{title: "API Documentation", segment: "api-docs"},
			{title: "CLI Reference", segment: "cli-reference"},
		]
	},
	{
		title: "Deploy"
		segment: "deploy"
		docs: [
			{title: "The Encore Platform", segment: "platform"},
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
