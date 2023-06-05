//go:build encore_app

package api

import "reflect"

func RegisterEndpoint(handler Handler) {
	Singleton.registerEndpoint(handler)
}

func RegisterAuthHandler(handler AuthHandler) {
	Singleton.setAuthHandler(handler)
}

func RegisterAuthDataType[T any]() {
	var zero T
	RegisteredAuthDataType = reflect.TypeOf(zero)
}

func RegisterGlobalMiddleware(mw *Middleware) {
	Singleton.registerGlobalMiddleware(mw)
}
