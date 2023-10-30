//go:build encore_app

package api

import "reflect"

func RegisterEndpoint(handler Handler) {
	Singleton.registerEndpoint(handler)
}

func RegisterAuthHandler(handler AuthHandler) {
	Singleton.setAuthHandler(handler)
}

// RegisterAuthDataType registers the type of the auth data that will be
// returned by the auth handler. This is used to verify that the auth data
// returned by the auth handler is of the correct type.
//
// Note type T is required to be a pointer type.
func RegisterAuthDataType[T any]() {
	var zero T
	RegisteredAuthDataType = reflect.TypeOf(zero)
}

func RegisterGlobalMiddleware(mw *Middleware) {
	Singleton.registerGlobalMiddleware(mw)
}
