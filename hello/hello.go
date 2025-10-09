// Service hello implements a simple hello world REST API.
package hello

import (
	"context"
)27

// Welcome to Encore!
// This is a simple "Hello World" project to get you started.
//
// To run it, execute "encore run" in your favorite shell.

// ==================================================================

// This is a public REST API that responds with a personalized greeting.
// Learn more about defining APIs with Encore:
// https://encore.dev/docs/primitives/services-and-apis
//
// To call it, run in your terminal:
//
//	curl http://localhost:4000/hello/World
//
//encore:api public path=/hello/:name
func World(ctx context.Context, name string) (*Response, error) {
	msg := "Hello, " + name + "!"
	return &Response{Message: msg}, nil
}

type Response struct {
	Message string
}

// ==================================================================

// Encore comes with a built-in local development dashboard for
// exploring your API, viewing documentation, debugging with
// distributed tracing, and more:
//
//     http://localhost:9400
//

// ==================================================================

// Next steps
//
// 1. Deploy your application to the cloud
//
//     git add -A .
//     git commit -m 'Commit message'
//     git push encore
//
// 2. To continue exploring Encore, check out some of these topics:
//
// 	  Defining Services:			 https://encore.dev/docs/go/primitives/services
// 	  Defining APIs:				 https://encore.dev/docs/go/primitives/defining-apis
//    Using SQL databases:  		 https://encore.dev/docs/go/primitives/databases
//    Using Pub/Sub:  				 https://encore.dev/docs/go/primitives/pubsub
//    Authenticating users: 		 https://encore.dev/docs/go/develop/auth
//    Building a REST API:  		 https://encore.dev/docs/go/tutorials/rest-api
//	  Building an Event-Driven app:  https://encore.dev/docs/go/tutorials/uptime
//    Building a Slack bot: 		 https://encore.dev/docs/go/tutorials/slack-bot
//	  Example apps repo:			 https://github.com/encoredev/examples
