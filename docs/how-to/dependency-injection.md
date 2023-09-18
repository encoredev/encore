---
seotitle: How to use dependency injection to test your microservices app
seodesc: Learn how to use dependency injection in your Go based microservices backend application using Encore.
title: Dependency Injection
subtitle: Simplifying testing
---

Dependency Injection is a fancy name for a simple concept: when you depend on some
functionality, add that dependency as a field on your struct and refer to it that way
instead of directly calling it. By doing so it becomes easier to test your services
by swapping out certain dependencies for other implementations (often with the use of
interfaces).

Encore provides built-in support for dependency injection in services through the use
of the `//encore:service` directive and a **service struct**. See the [service structs docs](/docs/primitives/services-and-apis/service-structs) more information on how to define service structs.

As an example, consider an email service that has a SendGrid API client that is
dependency injected. It might look like this:

```go
package email

//encore:service
type Service struct {
	sendgridClient *sendgrid.Client
}

func initService() (*Service, error) {
    client, err := sendgrid.NewClient()
    if err != nil {
        return nil, err
    }
    return &Service{sendgridClient: client}, nil
}
```

You can then define APIs as methods on this struct:
```go
//encore:api private
func (s *Service) Send(ctx context.Context, p *SendParams) error {
	// ... use s.sendgridClient to send emails ...
}
```

### Mocking dependencies

If you wish to mock out the SendGrid client for testing purposes you can change the
field to an interface:

```go
type sendgridClient interface {
	SendEmail(...) // a hypothetical signature, for illustration purposes
}

//encore:service
type Service struct {
    sendgridClient sendgridClient
}
```

Then during your tests you can instantiate the service object by hand:
```go
func TestFoo(t *testing.T) {
    svc := &Service{sendgridClient: &myMockClient{}}
    // ...
}
```
