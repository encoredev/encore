package types

import "net/http"

type SubscriptionID string

type PushEndpointHandler func(req *http.Request) error

type PushEndpointRegistry interface {
	RegisterPushSubscriptionHandler(id SubscriptionID, handler PushEndpointHandler)
}
