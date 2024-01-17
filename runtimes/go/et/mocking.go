//go:build encore_app

package et

import (
	"fmt"
)

// MockAPI allows you to mock out an API in your tests; Any calls made to the API
// during this test or any of its sub-tests will be routed to the mock you provide.
//
// Your mocked function must match the signature of the API you are mocking.
//
// For example, if you have an API defined as:
//
//	//encore:api public
//	func MyAPI(ctx context.Context, req *MyAPIRequest) (*MyAPIResponse, error) {
//		...
//	}
//
// You can mock it out in your test as:
//
//	et.MockAPI("mysvc", "MyAPI", func(ctx context.Context, req *MyAPIRequest) (*MyAPIResponse, error) {
//		...
//	})
//
// Note: if you use [MockService] to mock a service and then use this function to mock
// an API on that service, the API mock will take precedence over the service mock.
//
// Setting the mock to nil will remove the API mock
func MockAPI(serviceName string, apiName string, mock any) {
	if !Singleton.server.EndpointExists(serviceName, apiName) {
		panic(fmt.Sprintf("cannot mock API %s.%s: API does not exist", serviceName, apiName))
	}

	Singleton.testMgr.SetAPIMock(serviceName, apiName, mock)
}

// MockService allows you to mock out a service in your tests; Any calls made to the service
// during this test or any of its sub-tests will be routed to the mock you provide.
//
// Your mock must implement the all the API methods of the service which are used during the
// test(s). If you do not implement a method, it will panic when that method is called.
//
// If you want to ensure compile time safety, you can use the Interface type generated for
// the service, which will ensure that you implement all the methods. For example:
//
//	package svca
//
//	import (
//		"testing"
//		"encore.dev/et"
//
//		"encore.app/svcb"
//	)
//
//	func TestServiceA(t *testing.T) {
//		et.MockService[svcb.Interface]("svcb", &myMockType{})
//		SomeFuncInThisPackageWhichUltimatelyCallsServiceB()
//	}
//
// Setting the mock to nil will remove the service mock
func MockService[T any](serviceName string, mock T) {
	if !Singleton.server.ServiceExists(serviceName) {
		panic(fmt.Sprintf("cannot mock service %s: service does not exist", serviceName))
	}

	Singleton.testMgr.SetServiceMock(serviceName, mock)
}
