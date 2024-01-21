---
seotitle: Mocking out your APIs and services for testing
seodesc: Learn how to mock out your APIs and services for testing, and how to use the built-in mocking support in Encore.
title: Mocking
subtitle: Testing your application in isolation
infobox: {
  title: "Testing",
  import: "encore.dev/et",
}
---

Encore comes with built-in support for mocking out APIs and services, which makes it easier to test your application in
isolation.

## Mocking Endpoints

Let's say you have an endpoint that calls an external API in our `products` service:

```go
//encore:api private
func GetPrice(ctx context.Context, p *PriceParams) (*PriceResponse, error) {
    // Call external API to get the price
}
```

When testing this function, you don't want to call the real external API since that would be slow and cause your tests
to fail if the API is down. Instead, you want to mock out the API call and return a fake response.

In Encore, you can do this by adding a mock implementation of the endpoint using the `et.MockEndpoint` function inside your test:

```go
package shoppingcart

import (
	"context"
	"testing"
	
	"encore.dev/et" // Encore's test support package
	
	"your_app/products"
)


func Test_Something(t *testing.T) {
	t.Parallel() // Run this test in parallel with other tests without the mock implementation interfering
	
	// Create a mock implementation of pricing API which will only impact this test and any sub-tests
	et.MockEndpoint(products.GetPrice, func(ctx context.Context, p *products.PriceParams) (*products.PriceResponse, error) {
		return &products.PriceResponse{Price: 100}, nil
	})
	
	// ... the rest of your test code here ...
} 
```

When any code within the test, or any sub-test calls the `GetPrice` API, the mock implementation will be called instead.
The mock will not impact any other tests running in parallel. The function you pass to `et.MockEndpoint` must have the same
signature as the real endpoint.

If you want to mock out the API for all tests in the package, you can add the mock implementation to the `TestMain` function:

```go
package shoppingcart

import (
	"context"
	"os"
    "testing"
    
    "encore.dev/et"
	
	"your_app/products"
)

func TestMain(m *testing.M) {
    // Create a mock implementation of pricing API which will impact all tests within this package
    et.MockEndpoint(products.GetPrice, func(ctx context.Context, p *products.PriceParams) (*products.PriceResponse, error) {
        return &products.PriceResponse{Price: 100}, nil
    })
    
    // Now run the tests
    os.Exit(m.Run())
}
```

Mocks can be changed at any time, including removing them by setting the mock implementation to `nil`.

## Mocking services

As well as mocking individual APIs, you can also mock entire services. This can be useful if you want to inject a different
set of dependencies into your service for testing, or a service that your code depends on. This can be done using the
`et.MockService` function:

```go
package shoppingcart

import (
    "context"
    "testing"
    
    "encore.dev/et" // Encore's test support package
    
    "your_app/products"
)

func Test_Something(t *testing.T) {
    t.Parallel() // Run this test in parallel with other tests without the mock implementation interfering
    
    // Create a instance of the products service which will only impact this test and any sub-tests
    et.MockService("products", &products.Service{
		SomeField: "a testing value",
	})
    
    // ... the rest of your test code here ...
}
```

When any code within the test, or any sub-test calls the `products` service, the mock implementation will be called instead.
Unlike `et.MockEndpoint`, the mock implementation does not need to have the same signature, and can be any object. The only requirement
is that any of the services APIs that are called during the test must be implemented by as a receiver method on the mock object.
(This also includes APIs that are defined as package level functions in the service, and are not necessarily defined as receiver methods
on that services struct).

To help with compile time safety on service mocking, for every service Encore will automatically generate an `Interface` interface
which contains all the APIs defined in the service. This interface can be passed as a generic argument to `et.MockService` to ensure
that the mock object implements all the APIs defined in the service:

```go
type myMockObject struct{}

func (m *myMockObject) GetPrice(ctx context.Context, p *products.PriceParams) (*products.PriceResponse, error) {
    return &products.PriceResponse{Price: 100}, nil
}

func Test_Something(t *testing.T) {
    t.Parallel() // Run this test in parallel with other tests without the mock implementation interfering
    
    // This will cause a compile time error if myMockObject does not implement all the APIs defined in the products service
    et.MockService[products.Interface]("products", &myMockObject{})
}
```

### Automatic generation of mock objects

Thanks to the generated `Interface` interface, it's possible to automatically generate mock objects for your services using
either [Mockery](https://vektra.github.io/mockery/latest/) or [GoMock](https://github.com/uber-go/mock).
