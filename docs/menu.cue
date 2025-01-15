#Menu: #RootMenu | #SubMenu

#RootMenu: {
	kind: "rootmenu"
	items: [...#MenuItem]
}

#SubMenu: {
	kind: "submenu"
	// Menu title to display when this submenu is active.
	title: string

	// ID for the submenu, used for tracking active menu in frontend.
	id: string

	// Additional presentation options for the menu item.
	presentation?: #Presentation

	back: {
		// Text to display in the back button.
		text: string

		// Path to the page to navigate to when the back button is clicked.
		path: string
	}

	items: [...#MenuItem]
}

// Represents an item in a menu.
#MenuItem: #SectionMenuItem | #BasicMenuItem | #NavMenuItem | #AccordionMenuItem

#SectionMenuItem: {
	// Represents a menu section that can't be navigated to.
	kind: "section"

	// The text to display in the menu.
	text: string

	// Menu items to show for this section.
	items: [...#MenuItem]
}

#BasicMenuItem: {
	// Represents a basic page that can be navigated to.
	kind: "basic"

	// The text to display in the menu.
	text: string

	// The URL path to the page.
	path: string

	// The file to render when viewing this page.
	file: string

	// Inline menu to show when viewing this page.
	inline_menu?: [...#MenuItem]

	// hidden, if true, indicates the page exists but is hidden in the menu.
	// It can be navigated to directly, and will be show as "next page"/"prev page"
	// in the per-page navigation.
	hidden?: true
}

#NavMenuItem: {
	// Represents a page that can be navigated to, that has a menu
	// that replaces the navigation when viewing this page.
	kind: "nav"

	// The text to display in the menu.
	text: string

	// The URL path to the page.
	path: string

	// The file to render when viewing this page.
	file: string

	// The items to display in the submenu.
	submenu: #SubMenu

	// Additional presentation options for the menu item.
	presentation?: #Presentation
}

#Presentation: {
	// Icon to display next to the menu item.
	icon?: string
	style: "card" | *"basic"
}

#AccordionMenuItem: {
	kind: "accordion"
	text: string
	// If the accordion is open by default.
	defaultExpanded: bool | *false

	// The items to display in the accordion.
	accordion: [...#MenuItem]
}

// The root object is a #RootMenu.
#RootMenu
{
	items: [
		{
			kind:    "nav"
			text:    "Encore.go"
			path:    "/go"
			file:    "go/overview"
			submenu: #EncoreGO
			presentation: {
				icon:  "golang"
				style: "card"
			}
		}, {
			kind:    "nav"
			text:    "Encore.ts"
			path:    "/ts"
			file:    "ts/overview"
			submenu: #EncoreTS
			presentation: {
				icon:  "typescript"
				style: "card"
			}
		}, {
			kind:    "nav"
			text:    "Encore Cloud"
			path:    "/platform"
			file:    "platform/overview"
			submenu: #EncorePlatform
			presentation: {
				icon:  "typescript"
				style: "card"
			}
		},
	]
}

#EncoreGO: #SubMenu & {
	title: "Encore.go"
	id:    "go"
	presentation: {
		icon: "golang"
	}
	back: {
		text: ""
		path: ""
	}
	items: [
		{
			kind: "section"
			text: "Get Started"
			items: [{
				kind: "basic"
				text: "Installation"
				path: "/go/install"
				file: "go/install"
			}, {
				kind: "basic"
				text: "Quick Start"
				path: "/go/quick-start"
				file: "go/quick-start"
			}, {
				kind: "basic"
				text: "FAQ"
				path: "/go/faq"
				file: "go/faq"
			}]
		},
		{
			kind: "section"
			text: "Concepts"
			items: [{
				kind: "basic"
				text: "Benefits"
				path: "/go/concepts/benefits"
				file: "go/concepts/benefits"
			}, {
				kind: "basic"
				text: "Application Model"
				path: "/go/concepts/application-model"
				file: "go/concepts/application-model"
			}]
		},
		{
			kind: "section"
			text: "Tutorials"
			items: [{
				kind: "basic"
				text: "Building a REST API"
				path: "/go/tutorials/rest-api"
				file: "go/tutorials/rest-api"
			}, {
				kind: "basic"
				text: "Building an Uptime Monitor"
				path: "/go/tutorials/uptime"
				file: "go/tutorials/uptime"
			}, {
				kind: "basic"
				text: "Building a GraphQL API"
				path: "/go/tutorials/graphql"
				file: "go/tutorials/graphql"
			}, {
				kind: "basic"
				text: "Building a Slack bot"
				path: "/go/tutorials/slack-bot"
				file: "go/tutorials/slack-bot"
			}, {
				kind: "basic"
				text: "Building a Meeting Notes app"
				path: "/go/tutorials/meeting-notes"
				file: "go/tutorials/meeting-notes"
			}, {
				kind: "basic"
				text: "Building a Booking System"
				path: "/go/tutorials/booking-system"
				file: "go/tutorials/booking-system"
			}, {
				kind:   "basic"
				text:   "Building an Incident Management tool"
				path:   "/go/tutorials/incident-management-tool"
				file:   "go/tutorials/incident-management-tool"
				hidden: true
			}]
		},
		{
			kind: "section"
			text: "Primitives"
			items: [{
				kind: "basic"
				text: "App Structure"
				path: "/go/primitives/app-structure"
				file: "go/primitives/app-structure"
			}, {
				kind: "basic"
				text: "Services"
				path: "/go/primitives/services"
				file: "go/primitives/services"
			}, {
				kind: "accordion"
				text: "APIs"
				accordion: [{
					kind: "basic"
					text: "Defining APIs"
					path: "/go/primitives/defining-apis"
					file: "go/primitives/defining-apis"
				}, {
					kind: "basic"
					text: "API Calls"
					path: "/go/primitives/api-calls"
					file: "go/primitives/api-calls"
				}, {
					kind: "basic"
					text: "Raw Endpoints"
					path: "/go/primitives/raw-endpoints"
					file: "go/primitives/raw-endpoints"
				}, {
					kind: "basic"
					text: "Service Structs"
					path: "/go/primitives/service-structs"
					file: "go/primitives/service-structs"
				}, {
					kind: "basic"
					text: "API Errors"
					path: "/go/primitives/api-errors"
					file: "go/primitives/api-errors"
				}]
			}, {
				kind: "accordion"
				text: "Databases"
				accordion: [{
					kind: "basic"
					text: "Using SQL databases"
					path: "/go/primitives/databases"
					file: "go/primitives/databases"
				}, {
					kind: "basic"
					text: "Change SQL database schema"
					path: "/go/primitives/change-db-schema"
					file: "go/primitives/change-db-schema"
				}, {
					kind: "basic"
					text: "Integrate with existing databases"
					path: "/go/primitives/connect-existing-db"
					file: "go/primitives/connect-existing-db"
				}, {
					kind: "basic"
					text: "Insert test data in a database"
					path: "/go/primitives/insert-test-data-db"
					file: "go/primitives/insert-test-data-db"
				}, {
					kind: "basic"
					text: "Share databases between services"
					path: "/go/primitives/share-db-between-services"
					file: "go/primitives/share-db-between-services"
				}, {
					kind: "basic"
					text: "PostgreSQL Extensions"
					path: "/go/primitives/databases/extensions"
					file: "go/primitives/database-extensions"
				}, {
					kind: "basic"
					text: "Troubleshooting"
					path: "/go/primitives/databases/troubleshooting"
					file: "go/primitives/database-troubleshooting"
				}]
			}, {
				kind: "basic"
				text: "Object Storage"
				path: "/go/primitives/object-storage"
				file: "go/primitives/object-storage"
			}, {
				kind: "basic"
				text: "Cron Jobs"
				path: "/go/primitives/cron-jobs"
				file: "go/primitives/cron-jobs"
			}, {
				kind: "basic"
				text: "Pub/Sub"
				path: "/go/primitives/pubsub"
				file: "go/primitives/pubsub"
			}, {
				kind: "basic"
				text: "Caching"
				path: "/go/primitives/caching"
				file: "go/primitives/caching"
			}, {
				kind: "basic"
				text: "Secrets"
				path: "/go/primitives/secrets"
				file: "go/primitives/secrets"
			}, {
				kind: "basic"
				text: "Code Snippets"
				path: "/go/primitives/code-snippets"
				file: "go/primitives/code-snippets"
			}]
		}, {
			kind: "section"
			text: "Development"
			items: [{
				kind: "basic"
				text: "Authentication"
				path: "/go/develop/auth"
				file: "go/develop/auth"
			}, {
				kind: "basic"
				text: "Configuration"
				path: "/go/develop/config"
				file: "go/develop/config"
			}, {
				kind: "basic"
				text: "CORS"
				path: "/go/develop/cors"
				file: "go/develop/cors"
			}, {
				kind: "basic"
				text: "Metadata"
				path: "/go/develop/metadata"
				file: "go/develop/metadata"
			}, {
				kind: "basic"
				text: "Middleware"
				path: "/go/develop/middleware"
				file: "go/develop/middleware"
			}, {
				kind: "basic"
				text: "Testing"
				path: "/go/develop/testing"
				file: "go/develop/testing"
			}, {
				kind: "basic"
				text: "Mocking"
				path: "/go/develop/testing/mocking"
				file: "go/develop/mocking"
			}, {
				kind: "basic"
				text: "Validation"
				path: "/go/develop/validation"
				file: "go/develop/validation"
			}]
		},
		{
			kind: "section"
			text: "CLI"
			items: [{
				kind: "basic"
				text: "CLI Reference"
				path: "/go/cli/cli-reference"
				file: "go/cli/cli-reference"
			}, {
				kind: "basic"
				text: "Client Generation"
				path: "/go/cli/client-generation"
				file: "go/cli/client-generation"
			}, {
				kind: "basic"
				text: "Infra Namespaces"
				path: "/go/cli/infra-namespaces"
				file: "go/cli/infra-namespaces"
			}, {
				kind: "basic"
				text: "CLI Configuration"
				path: "/go/cli/config-reference"
				file: "go/cli/config-reference"
			}, {
				kind: "basic"
				text: "Telemetry"
				path: "/go/cli/telemetry"
				file: "go/cli/telemetry"
			}]
		},
		{
			kind: "section"
			text: "Observability"
			items: [{
				kind: "basic"
				text: "Development Dashboard"
				path: "/go/observability/dev-dash"
				file: "go/observability/dev-dash"
			}, {
				kind: "basic"
				text: "Distributed Tracing"
				path: "/go/observability/tracing"
				file: "go/observability/tracing"
			}, {
				kind: "basic"
				text: "Flow Architecture Diagram"
				path: "/go/observability/encore-flow"
				file: "go/observability/encore-flow"
			}, {
				kind: "basic"
				text: "Service Catalog"
				path: "/go/observability/service-catalog"
				file: "go/observability/service-catalog"
			}, {
				kind: "basic"
				text: "Logging"
				path: "/go/observability/logging"
				file: "go/observability/logging"
			}]
		},
		{
			kind: "section"
			text: "Self Hosting"
			items: [
				{
					kind: "basic"
					text: "CI/CD"
					path: "/go/self-host/ci-cd"
					file: "go/self-host/ci-cd"
				},
				{
					kind: "basic"
					text: "Build Docker Images"
					path: "/go/self-host/docker-build"
					file: "go/self-host/self-host"
				}, {
					kind: "basic"
					text: "Configure Infrastructure"
					path: "/go/self-host/configure-infra"
					file: "go/self-host/configure-infra"
				}]
		},
		{
			kind: "section"
			text: "How to guides"
			items: [{
				kind: "basic"
				text: "Break a monolith into microservices"
				path: "/go/how-to/break-up-monolith"
				file: "go/how-to/break-up-monolith"
			}, {
				kind: "basic"
				text: "Integrate with a web frontend"
				path: "/go/how-to/integrate-frontend"
				file: "go/how-to/integrate-frontend"
			}, {
				kind: "basic"
				text: "Use Temporal with Encore"
				path: "/go/how-to/temporal"
				file: "go/how-to/temporal"
			}, {
				kind: "basic"
				text: "Build with cgo"
				path: "/go/how-to/cgo"
				file: "go/how-to/cgo"
			}, {
				kind: "basic"
				text: "Debug with Delve"
				path: "/go/how-to/debug"
				file: "go/how-to/debug"
			}, {
				kind: "basic"
				text: "Receive regular HTTP requests & Use websockets"
				path: "/go/how-to/http-requests"
				file: "go/how-to/http-requests"
			}, {
				kind: "basic"
				text: "Use Atlas + GORM for database migrations"
				path: "/go/how-to/atlas-gorm"
				file: "go/how-to/atlas-gorm"
			}, {
				kind: "basic"
				text: "Use the ent ORM for migrations"
				path: "/go/how-to/entgo-orm"
				file: "go/how-to/entgo-orm"
			}, {
				kind: "basic"
				text: "Use Connect for gRPC communication"
				path: "/go/how-to/grpc-connect"
				file: "go/how-to/grpc-connect"
			}, {
				kind: "basic"
				text: "Use a Pub/Sub Transactional Outbox"
				path: "/go/how-to/pubsub-outbox"
				file: "go/how-to/pubsub-outbox"
			}, {
				kind: "basic"
				text: "Use Dependency Injection"
				path: "/go/how-to/dependency-injection"
				file: "go/how-to/dependency-injection"
			}, {
				kind: "basic"
				text: "Use Auth0 Authentication"
				path: "/go/how-to/auth0-auth"
				file: "go/how-to/auth0-auth"
			}, {
				kind: "basic"
				text: "Use Clerk Authentication"
				path: "/go/how-to/clerk-auth"
				file: "go/how-to/clerk-auth"
			}, {
				kind: "basic"
				text: "Use Firebase Authentication"
				path: "/go/how-to/firebase-auth"
				file: "go/how-to/firebase-auth"
			}]
		},
		{
			kind: "section"
			text: "Migration guides"
			items: [{
				kind: "basic"
				text: "Migrate away from Encore"
				path: "/go/migration/migrate-away"
				file: "go/migration/migrate-away"
			}]
		},
		{
			kind: "section"
			text: "Community"
			items: [{
				kind: "basic"
				text: "Get Involved"
				path: "/go/community/get-involved"
				file: "go/community/get-involved"
			}, {
				kind: "basic"
				text: "Contribute"
				path: "/go/community/contribute"
				file: "go/community/contribute"
			}, {
				kind: "basic"
				text: "Open Source"
				path: "/go/community/open-source"
				file: "go/community/open-source"
			}, {
				kind: "basic"
				text: "Principles"
				path: "/go/community/principles"
				file: "go/community/principles"
			}, {
				kind: "basic"
				text: "Submit Template"
				path: "/go/community/submit-template"
				file: "go/community/submit-template"
			}]
		},
	]
}

#EncoreTS: #SubMenu & {
	title: "Encore.ts"
	id:    "ts"
	presentation: {
		icon: "typescript"
	}
	back: {
		text: ""
		path: ""
	}
	items: [
		{
			kind: "section"
			text: "Get started"
			items: [{
				kind: "basic"
				text: "Installation"
				path: "/ts/install"
				file: "ts/install"
			}, {
				kind: "basic"
				text: "Quick Start"
				path: "/ts/quick-start"
				file: "ts/quick-start"
			}, {
				kind: "basic"
				text: "FAQ"
				path: "/ts/faq"
				file: "ts/faq"
			}]
		},
		{
			kind: "section"
			text: "Concepts"
			items: [{
				kind: "basic"
				text: "Benefits"
				path: "/ts/concepts/benefits"
				file: "ts/concepts/benefits"
			}, {
				kind: "basic"
				text: "Application Model"
				path: "/ts/concepts/application-model"
				file: "ts/concepts/application-model"
			}, {
				kind: "basic"
				text: "Hello World"
				path: "/ts/concepts/hello-world"
				file: "ts/concepts/hello-world"
			}]
		},
		{
			kind: "section"
			text: "Tutorials"
			items: [{
				kind: "basic"
				text: "Building a REST API"
				path: "/ts/tutorials/rest-api"
				file: "ts/tutorials/rest-api"
			}, {
				kind: "basic"
				text: "Building an Uptime Monitor"
				path: "/ts/tutorials/uptime"
				file: "ts/tutorials/uptime"
			}, {
				kind: "basic"
				text: "Building a GraphQL API"
				path: "/ts/tutorials/graphql"
				file: "ts/tutorials/graphql"
			}, {
				kind: "basic"
				text: "Building a Slack bot"
				path: "/ts/tutorials/slack-bot"
				file: "ts/tutorials/slack-bot"
			}]
		},
		{
			kind: "section"
			text: "Primitives"
			items: [{
				kind: "basic"
				text: "App Structure"
				path: "/ts/primitives/app-structure"
				file: "ts/primitives/app-structure"
			}, {
				kind: "basic"
				text: "Services"
				path: "/ts/primitives/services"
				file: "ts/primitives/services"
			}, {
				kind: "accordion"
				text: "APIs"
				accordion: [{
					kind: "basic"
					text: "Defining APIs"
					path: "/ts/primitives/defining-apis"
					file: "ts/primitives/defining-apis"
				}, {
					kind: "basic"
					text: "Validation"
					path: "/ts/primitives/validation"
					file: "ts/primitives/validation"
				}, {
					kind: "basic"
					text: "API Calls"
					path: "/ts/primitives/api-calls"
					file: "ts/primitives/api-calls"
				}, {
					kind: "basic"
					text: "Raw Endpoints"
					path: "/ts/primitives/raw-endpoints"
					file: "ts/primitives/raw-endpoints"
				}, {
					kind: "basic"
					text: "GraphQL"
					path: "/ts/primitives/graphql"
					file: "ts/primitives/graphql"
				}, {
					kind: "basic"
					text: "Streaming APIs"
					path: "/ts/primitives/streaming-apis"
					file: "ts/primitives/streaming-apis"
				}, {
					kind: "basic"
					text: "API Errors"
					path: "/ts/primitives/errors"
					file: "ts/primitives/errors"
				}, {
					kind: "basic"
					text: "Static Assets"
					path: "/ts/primitives/static-assets"
					file: "ts/primitives/static-assets"
				}]
			}, {
				kind: "basic"
				text: "Databases"
				path: "/ts/primitives/databases"
				file: "ts/primitives/databases"
			}, {
				kind: "basic"
				text: "PostgreSQL Extensions"
				path: "/ts/primitives/databases-extensions"
				file: "ts/primitives/database-extensions"
			}, {
				kind: "basic"
				text: "Object Storage"
				path: "/ts/primitives/object-storage"
				file: "ts/primitives/object-storage"
			}, {
				kind: "basic"
				text: "Cron Jobs"
				path: "/ts/primitives/cron-jobs"
				file: "ts/primitives/cron-jobs"
			}, {
				kind: "basic"
				text: "Pub/Sub"
				path: "/ts/primitives/pubsub"
				file: "ts/primitives/pubsub"
			}, {
				kind: "basic"
				text: "Secrets"
				path: "/ts/primitives/secrets"
				file: "ts/primitives/secrets"
			}]
		}, {
			kind: "section"
			text: "Development"
			items: [{
				kind: "basic"
				text: "Authentication"
				path: "/ts/develop/auth"
				file: "ts/develop/auth"
			}, {
				kind: "accordion"
				text: "ORMs"
				accordion: [{
					kind: "basic"
					text: "Overview"
					path: "/ts/develop/orms"
					file: "ts/develop/orms/overview"
				}, {
					kind: "basic"
					text: "Knex.js"
					path: "/ts/develop/orms/knex"
					file: "ts/develop/orms/knex"
				}, {
					kind: "basic"
					text: "Prisma"
					path: "/ts/develop/orms/prisma"
					file: "ts/develop/orms/prisma"
				}, {
					kind: "basic"
					text: "Drizzle"
					path: "/ts/develop/orms/drizzle"
					file: "ts/develop/orms/drizzle"
				}, {
					kind: "basic"
					text: "Sequelize"
					path: "/ts/develop/orms/sequelize"
					file: "ts/develop/orms/sequelize"
				}]
			}, {
				kind: "basic"
				text: "Metadata"
				path: "/ts/develop/metadata"
				file: "ts/develop/metadata"
			}, {
				kind: "basic"
				text: "Testing"
				path: "/ts/develop/testing"
				file: "ts/develop/testing"
			}, {
				kind: "basic"
				text: "Debugging"
				path: "/ts/develop/debug"
				file: "ts/develop/debug"
			}, {
				kind: "basic"
				text: "Middleware"
				path: "/ts/develop/middleware"
				file: "ts/develop/middleware"
			}, {
				kind: "basic"
				text: "Multithreading"
				path: "/ts/develop/multithreading"
				file: "ts/develop/multithreading"
			}]
		},
		{
			kind: "section"
			text: "CLI"
			items: [{
				kind: "basic"
				text: "CLI Reference"
				path: "/ts/cli/cli-reference"
				file: "ts/cli/cli-reference"
			}, {
				kind: "basic"
				text: "Client Generation"
				path: "/ts/cli/client-generation"
				file: "ts/cli/client-generation"
			}, {
				kind: "basic"
				text: "Infra Namespaces"
				path: "/ts/cli/infra-namespaces"
				file: "ts/cli/infra-namespaces"
			}, {
				kind: "basic"
				text: "CLI Configuration"
				path: "/ts/cli/config-reference"
				file: "ts/cli/config-reference"
			}, {
				kind: "basic"
				text: "Telemetry"
				path: "/ts/cli/telemetry"
				file: "ts/cli/telemetry"
			}]
		},
		{
			kind: "section"
			text: "Frontend"
			items: [{
				kind: "basic"
				text: "Hosting"
				path: "/ts/frontend/hosting"
				file: "ts/frontend/hosting"
			}, {
				kind: "basic"
				text: "CORS"
				path: "/ts/frontend/cors"
				file: "ts/frontend/cors"
			}, {
				kind: "basic"
				text: "Request Client"
				path: "/ts/frontend/request-client"
				file: "ts/frontend/request-client"
			}, {
				kind: "basic"
				text: "Template Engine"
				path: "/ts/frontend/template-engine"
				file: "ts/frontend/template-engine"
			}, {
				kind: "basic"
				text: "Mono vs Multi Repo"
				path: "/ts/frontend/mono-vs-multi-repo"
				file: "ts/frontend/mono-vs-multi-repo"
			}]
		},
		{
			kind: "section"
			text: "Observability"
			items: [{
				kind: "basic"
				text: "Development Dashboard"
				path: "/ts/observability/dev-dash"
				file: "ts/observability/dev-dash"
			}, {
				kind: "basic"
				text: "Logging"
				path: "/ts/observability/logging"
				file: "ts/observability/logging"
			}, {
				kind: "basic"
				text: "Distributed Tracing"
				path: "/ts/observability/tracing"
				file: "ts/observability/tracing"
			}, {
				kind: "basic"
				text: "Flow Architecture Diagram"
				path: "/ts/observability/flow"
				file: "ts/observability/flow"
			}, {
				kind: "basic"
				text: "Service Catalog"
				path: "/ts/observability/service-catalog"
				file: "ts/observability/service-catalog"
			}]
		},
		{
			kind: "section"
			text: "Self Hosting"
			items: [
				{
					kind: "basic"
					text: "CI/CD"
					path: "/ts/self-host/ci-cd"
					file: "ts/self-host/ci-cd"
				},
				{
					kind: "basic"
					text: "Build Docker Images"
					path: "/ts/self-host/build"
					file: "ts/self-host/build"
				}, {
					kind: "basic"
					text: "Configure Infrastructure"
					path: "/ts/self-host/configure-infra"
					file: "ts/self-host/configure-infra"
				}, {
					kind: "basic"
					text: "Deploy to DigitalOcean"
					path: "/ts/self-host/deploy-digitalocean"
					file: "ts/self-host/deploy-to-digital-ocean"
				}, {
					kind: "basic"
					text: "Deploy to Railway"
					path: "/ts/self-host/deploy-railway"
					file: "ts/self-host/deploy-to-railway"
				}]
		},
		{
			kind: "section"
			text: "How to guides"
			items: [{
				kind: "basic"
				text: "Handle file uploads"
				path: "/ts/how-to/file-uploads"
				file: "ts/how-to/file-uploads"
			}, {
				kind: "basic"
				text: "Use NestJS with Encore"
				path: "/ts/how-to/nestjs"
				file: "ts/how-to/nestjs"
			}]
		}, {
			kind: "section"
			text: "Migration guides"
			items: [{
				kind: "basic"
				text: "Migrate away from Encore"
				path: "/ts/migration/migrate-away"
				file: "ts/migration/migrate-away"
			}, {
				kind: "basic"
				text: "Migrate from Express.js"
				path: "/ts/migration/express-migration"
				file: "ts/migration/express-migration"
			}]
		},
		{
			kind: "section"
			text: "Community"
			items: [{
				kind: "basic"
				text: "Get Involved"
				path: "/ts/community/get-involved"
				file: "ts/community/get-involved"
			}, {
				kind: "basic"
				text: "Contribute"
				path: "/ts/community/contribute"
				file: "ts/community/contribute"
			}, {
				kind: "basic"
				text: "Open Source"
				path: "/ts/community/open-source"
				file: "ts/community/open-source"
			}, {
				kind: "basic"
				text: "Principles"
				path: "/ts/community/principles"
				file: "ts/community/principles"
			}, {
				kind: "basic"
				text: "Submit Template"
				path: "/ts/community/submit-template"
				file: "ts/community/submit-template"
			}]
		},
	]
}

#EncorePlatform: #SubMenu & {
	title: "Encore Cloud"
	id:    "platform"
	presentation: {
		icon: ""
	}
	back: {
		text: ""
		path: ""
	}
	items: [
		{
			kind: "section"
			text: "Concepts"
			items: [{
				kind: "basic"
				text: "Introduction"
				path: "/platform/introduction"
				file: "platform/introduction"
			}]
		},
		{
			kind: "section"
			text: "Deployment"
			items: [{
				kind: "basic"
				text: "Deploying & CI/CD"
				path: "/platform/deploy/deploying"
				file: "platform/deploy/deploying"
			}, {
				kind: "basic"
				text: "Connect your cloud account"
				path: "/platform/deploy/own-cloud"
				file: "platform/deploy/own-cloud"
			}, {
				kind: "basic"
				text: "Environments"
				path: "/platform/deploy/environments"
				file: "platform/deploy/environments"
			}, {
				kind: "basic"
				text: "Preview Environments"
				path: "/platform/deploy/preview-environments"
				file: "platform/deploy/preview-environments"
			}, {
				kind: "basic"
				text: "Application Security"
				path: "/platform/deploy/security"
				file: "platform/deploy/security"
			}]
		},
		{
			kind: "section"
			text: "Infrastructure"
			items: [{
				kind: "basic"
				text: "Infrastructure overview"
				path: "/platform/infrastructure/infra"
				file: "platform/infrastructure/infra"
			}, {
				kind: "basic"
				text: "GCP Infrastructure"
				path: "/platform/infrastructure/gcp"
				file: "platform/infrastructure/gcp"
			}, {
				kind: "basic"
				text: "AWS Infrastructure"
				path: "/platform/infrastructure/aws"
				file: "platform/infrastructure/aws"
			}, {
				kind: "accordion"
				text: "Kubernetes deployment"
				accordion: [{
					kind: "basic"
					text: "Deploying to a new cluster"
					path: "/platform/infrastructure/kubernetes"
					file: "platform/infrastructure/kubernetes"
				}, {
					kind: "basic"
					text: "Import an existing cluster"
					path: "/platform/infrastructure/import-kubernetes-cluster"
					file: "platform/infrastructure/import-kubernetes-cluster"
				}, {
					kind: "basic"
					text: "Configure kubectl"
					path: "/platform/infrastructure/configure-kubectl"
					file: "platform/infrastructure/configure-kubectl"
				}]
			}, {
				kind: "basic"
				text: "Neon Postgres"
				path: "/platform/infrastructure/neon"
				file: "platform/infrastructure/neon"
			}, {
				kind: "basic"
				text: "Cloudflare R2"
				path: "/platform/infrastructure/cloudflare"
				file: "platform/infrastructure/cloudflare"
			}, {
				kind: "basic"
				text: "Managing database users"
				path: "/platform/infrastructure/manage-db-users"
				file: "platform/infrastructure/manage-db-users"
			}]
		}, {
			kind: "section"
			text: "Observability"
			items: [{
				kind: "basic"
				text: "Metrics"
				path: "/platform/observability/metrics"
				file: "platform/observability/metrics"
			}, {
				kind: "basic"
				text: "Distributed Tracing"
				path: "/platform/observability/tracing"
				file: "platform/observability/tracing"
			}, {
				kind: "basic"
				text: "Flow Architecture Diagram"
				path: "/platform/observability/encore-flow"
				file: "platform/observability/encore-flow"
			}, {
				kind: "basic"
				text: "Service Catalog"
				path: "/platform/observability/service-catalog"
				file: "platform/observability/service-catalog"
			}]
		},
		{
			kind: "section"
			text: "Integrations"
			items: [{
				kind: "basic"
				text: "GitHub"
				path: "/platform/integrations/github"
				file: "platform/integrations/github"
			}, {
				kind: "basic"
				text: "Custom Domains"
				path: "/platform/integrations/custom-domains"
				file: "platform/integrations/custom-domains"
			}, {
				kind: "basic"
				text: "Webhooks"
				path: "/platform/integrations/webhooks"
				file: "platform/integrations/webhooks"
			}, {
				kind: "basic"
				text: "OAuth Clients"
				path: "/platform/integrations/oauth-clients"
				file: "platform/integrations/oauth-clients"
			}, {
				kind: "basic"
				text: "Auth Keys"
				path: "/platform/integrations/auth-keys"
				file: "platform/integrations/auth-keys"
			}, {
				kind: "basic"
				text: "API Reference"
				path: "/platform/integrations/api-reference"
				file: "platform/integrations/api-reference"
			}, {
				kind: "basic"
				text: "Terraform"
				path: "/platform/integrations/terraform"
				file: "platform/integrations/terraform"
			}]
		},
		{
			kind: "section"
			text: "Migration guides"
			items: [{
				kind: "basic"
				text: "Try Encore for an existing project"
				path: "/platform/migration/try-encore"
				file: "platform/migration/try-encore"
			}, {
				kind: "basic"
				text: "Migrate an existing backend to Encore"
				path: "/platform/migration/migrate-to-encore"
				file: "platform/migration/migrate-to-encore"
			}, {
				kind: "basic"
				text: "Migrate away from Encore"
				path: "/platform/migration/migrate-away"
				file: "platform/migration/migrate-away"
			}]
		},
		{
			kind: "section"
			text: "Management & Billing"
			items: [{
				kind: "basic"
				text: "Compliance & Security"
				path: "/platform/management/compliance"
				file: "platform/management/compliance"
			}, {
				kind: "basic"
				text: "Plans & billing"
				path: "/platform/management/billing"
				file: "platform/management/billing"
			}, {
				kind: "basic"
				text: "Telemetry"
				path: "/platform/management/telemetry"
				file: "platform/management/telemetry"
			}, {
				kind: "basic"
				text: "Roles & Permissions"
				path: "/platform/management/permissions"
				file: "platform/management/permissions"
			}, {
				kind: "basic"
				text: "Usage limits"
				path: "/platform/management/usage"
				file: "platform/management/usage"
			}]
		},
		{
			kind: "section"
			text: "Other"
			items: [
				{
					kind: "accordion"
					text: "Product comparisons"
					accordion: [{
						kind: "basic"
						text: "Encore vs. Heroku"
						path: "/platform/other/vs-heroku"
						file: "platform/other/vs-heroku"
					}, {
						kind: "basic"
						text: "Encore vs. Supabase / Firebase"
						path: "/platform/other/vs-supabase"
						file: "platform/other/vs-supabase"
					}, {
						kind: "basic"
						text: "Encore vs. Terraform / Pulumi"
						path: "/platform/other/vs-terraform"
						file: "platform/other/vs-terraform"
					}]
				}]
		},
	]
}
