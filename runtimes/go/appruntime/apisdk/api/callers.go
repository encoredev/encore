package api

import (
	"errors"
	"fmt"
	"strings"
)

// Caller is an interface which can be used to identify the caller of an RPC call.
// if the caller is an authenticated source.
type Caller interface {
	CallerString() string   // What is the caller wireformat?
	PrivateAPIAccess() bool // Is this caller allowed to access private APIs?
}

// ApiCaller is a caller which represents an RPC which made a service-to-service API call
type ApiCaller struct {
	ServiceName string
	Endpoint    string
}

func (s ApiCaller) CallerString() string {
	return fmt.Sprintf("api:%s.%s", s.ServiceName, s.Endpoint)
}

func (s ApiCaller) PrivateAPIAccess() bool {
	return true
}

// PubSubCaller is a caller which represents a PubSub subscription which made a service-to-service API call
type PubSubCaller struct {
	Topic        string
	Subscription string
	MessageID    string
}

func (p PubSubCaller) CallerString() string {
	return fmt.Sprintf("pubsub:%s:%s:%s", p.Topic, p.Subscription, p.MessageID)
}

func (p PubSubCaller) PrivateAPIAccess() bool {
	return true
}

// AppCaller is a caller which represents the app itself made a service-to-service API call, but outside any traced process
// - This most likely means the call was made from a background process or init function
type AppCaller struct {
	DeployID string
}

func (a AppCaller) CallerString() string {
	return fmt.Sprintf("app:%s", a.DeployID)
}

func (a AppCaller) PrivateAPIAccess() bool {
	return true
}

// GatewayCaller is a caller which represents an API gateway within the application made a service-to-service API call
// This is used to identify the fact that the call was made from a gateway, and not from a background process or init function
// and should not be allowed access to the private routes
type GatewayCaller struct {
	ServiceName string // The service name being called by the gateway
	Endpoint    string // The endpoint being called by the gateway
}

func (g GatewayCaller) CallerString() string {
	return fmt.Sprintf("gateway:%s.%s", g.ServiceName, g.Endpoint)
}

func (g GatewayCaller) PrivateAPIAccess() bool {
	return false
}

// EncoreCaller represents an RPC call made from Encore's central systems (such as the Cloud dashboard)
type EncoreCaller struct {
	Principal string // The principal which made the call, could be an end user or a service
}

func (e EncoreCaller) CallerString() string {
	return fmt.Sprintf("encore:%s", e.Principal)
}

func (e EncoreCaller) PrivateAPIAccess() bool {
	return true
}

// ParseCallerString parses a caller string into a Caller object
func ParseCallerString(callerStr string) (Caller, error) {
	switch {
	case strings.HasPrefix(callerStr, "api:"):
		service, endpoint, found := strings.Cut(callerStr[len("api:"):], ".")
		if !found {
			return nil, errors.New("invalid api caller")
		}
		return ApiCaller{service, endpoint}, nil
	case strings.HasPrefix(callerStr, "pubsub:"):
		topic, subscriptionAndMsgId, found := strings.Cut(callerStr[len("pubsub:"):], ":")
		if !found {
			return nil, errors.New("invalid pubsub caller")
		}
		subscription, msgId, found := strings.Cut(subscriptionAndMsgId, ":")
		if !found {
			return nil, errors.New("invalid pubsub caller")
		}
		return PubSubCaller{topic, subscription, msgId}, nil
	case strings.HasPrefix(callerStr, "app:"):
		return AppCaller{callerStr[len("app:"):]}, nil
	case strings.HasPrefix(callerStr, "gateway:"):
		service, endpoint, found := strings.Cut(callerStr[len("gateway:"):], ".")
		if !found {
			return nil, errors.New("invalid gateway caller")
		}
		return GatewayCaller{service, endpoint}, nil
	case strings.HasPrefix(callerStr, "encore:"):
		return EncoreCaller{callerStr[len("encore:"):]}, nil
	default:
		return nil, errors.New("invalid caller")
	}
}
