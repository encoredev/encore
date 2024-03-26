package ai

import (
	"context"
	"strings"
	"sync"

	"github.com/hasura/go-graphql-client"
	"github.com/hasura/go-graphql-client/pkg/jsonutil"

	"encr.dev/pkg/fns"
	"encr.dev/pkg/idents"
	meta "encr.dev/proto/encore/parser/meta/v1"
	"encr.dev/v2/parser/apis/api/apienc"
)

type Manager struct {
	client *LazyClient
}

func NewAIManager(client *graphql.SubscriptionClient) *Manager {
	return &Manager{client: newLazyClient(client)}
}

func newLazyClient(client *graphql.SubscriptionClient) *LazyClient {
	lazy := &LazyClient{
		SubscriptionClient: client,
		notifiers:          make(map[string]func([]byte, error) error),
	}
	client.OnDisconnected(func() {
		lazy.mu.Lock()
		defer lazy.mu.Unlock()
		lazy.running = false
	})
	client.OnConnected(func() {
		lazy.mu.Lock()
		defer lazy.mu.Unlock()
		lazy.running = true
	})
	client.OnSubscriptionComplete(func(sub graphql.Subscription) {
		lazy.mu.Lock()
		defer lazy.mu.Unlock()
		delete(lazy.notifiers, sub.GetKey())
	})
	return lazy
}

type LazyClient struct {
	*graphql.SubscriptionClient

	mu        sync.Mutex
	running   bool
	notifiers map[string]func([]byte, error) error
}

func (l *LazyClient) Subscribe(query interface{}, variables map[string]interface{}, notify func([]byte, error) error) (string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.running {
		go func() {
			defer l.Close()
			err := l.Run()
			l.mu.Lock()
			defer l.mu.Unlock()
			if err != nil {
				for _, n := range l.notifiers {
					_ = n(nil, err)
				}
			}
			l.notifiers = make(map[string]func([]byte, error) error)
		}()
	}
	subID, err := l.SubscriptionClient.Subscribe(query, variables, notify)
	if err != nil {
		return "", err
	}
	key := l.GetSubscription(subID).GetKey()
	l.notifiers[key] = notify
	return subID, nil
}

type genericQuery struct {
	StreamUpdate *StreamUpdate `graphql:"result"`
}

func query[T any](ctx context.Context, c *LazyClient, params map[string]interface{}, notifier AINotifier) (string, error) {
	var subId string
	var errStrReply = func(error string) error {
		_ = notifier(ctx, &WSNotification{
			SubscriptionID: subId,
			Error:          error,
			Finished:       true,
		})
		return graphql.ErrSubscriptionStopped
	}
	var errReply = func(err error) error {
		return errStrReply(err.Error())
	}
	var query T
	subId, err := c.Subscribe(&query, params, func(message []byte, err error) error {
		if err != nil {
			return errReply(err)
		}
		var result genericQuery
		err = jsonutil.UnmarshalGraphQL(message, &result)
		if err != nil {
			return errReply(err)
		}
		if result.StreamUpdate.Error != "" {
			return errStrReply(result.StreamUpdate.Error)
		}
		err = notifier(ctx, &WSNotification{
			SubscriptionID: subId,
			Value:          result.StreamUpdate.Value.GetValue(),
			Finished:       result.StreamUpdate.Finished,
		})
		if err != nil {
			return errReply(err)
		}

		return nil
	})
	c.GetSubscription(subId).GetKey()
	return subId, err
}

type UpdateQuery struct {
	Type string `graphql:"__typename"`

	ServiceUpdate   `graphql:"... on ServiceUpdate"`
	TypeUpdate      `graphql:"... on TypeUpdate"`
	TypeFieldUpdate `graphql:"... on TypeFieldUpdate"`
	ErrorUpdate     `graphql:"... on ErrorUpdate"`
	EndpointUpdate  `graphql:"... on EndpointUpdate"`
}

func (u *UpdateQuery) GetValue() AIUpdateType {
	switch u.Type {
	case "ServiceUpdate":
		return u.ServiceUpdate
	case "TypeUpdate":
		return u.TypeUpdate
	case "TypeFieldUpdate":
		return u.TypeFieldUpdate
	case "ErrorUpdate":
		return u.ErrorUpdate
	case "EndpointUpdate":
		return u.EndpointUpdate
	}
	return nil
}

type StreamUpdate struct {
	Value    UpdateQuery
	Error    string
	Finished bool
}

type WSNotification struct {
	SubscriptionID string `json:"subscriptionId,omitempty"`
	Value          any    `json:"value,omitempty"`
	Error          string `json:"error,omitempty"`
	Finished       bool   `json:"finished,omitempty"`
}

type AINotifier func(context.Context, *WSNotification) error

type endpointStructs struct {
	service    string
	endpoint   string
	defaultLoc apienc.WireLoc
	structs    []*TypeInput
}

func (e *endpointStructs) Notification() EndpointStructs {
	codes := fns.Map(e.structs, (*TypeInput).Render)
	return EndpointStructs{
		Service:  e.service,
		Type:     "EndpointStructs",
		Endpoint: e.endpoint,
		Types:    e.structs,
		Code:     strings.Join(codes, "\n"),
	}
}

func (e *endpointStructs) upsertStruct(name, doc string) *TypeInput {
	for _, s := range e.structs {
		if s.Name == name {
			if doc != "" {
				s.Doc = doc
			}
			return s
		}
	}
	si := &TypeInput{Name: name, Doc: doc}
	e.structs = append(e.structs, si)
	return si
}

func (e *endpointStructs) upsertField(up TypeFieldUpdate) *TypeInput {
	s := e.upsertStruct(up.Struct, "")
	for _, f := range s.Fields {
		if f.Name == up.Name {
			if up.Doc != "" {
				f.Doc = up.Doc
			}
			if up.Type != "" {
				f.Type = up.Type
			}
			return s
		}
	}
	fi := &TypeFieldInput{
		Name:     up.Name,
		Doc:      up.Doc,
		Type:     up.Type,
		Location: e.defaultLoc,
		WireName: idents.Convert(up.Name, idents.CamelCase),
	}
	s.Fields = append(s.Fields, fi)
	return s
}

type endpointCache struct {
	eps      map[string]*endpointStructs
	proposed []ServiceInput
}

func (s *endpointCache) endpoint(service, endpoint string) *endpointStructs {
	key := service + "." + endpoint
	if _, ok := s.eps[key]; !ok {
		for _, svc := range s.proposed {
			if svc.Name != service {
				continue
			}
			for _, ep := range svc.Endpoints {
				if ep.Name != endpoint {
					continue
				}
				defaultLoc := apienc.Body
				switch ep.Method {
				case "GET", "HEAD", "DELETE":
					defaultLoc = apienc.Query
				}
				s.eps[key] = &endpointStructs{
					service:    service,
					endpoint:   endpoint,
					defaultLoc: defaultLoc,
				}
				break
			}
			break
		}
		if s.eps[key] == nil {
			panic("endpoint not found")
		}
	}
	return s.eps[key]
}

func (m *Manager) DefineEndpoints(ctx context.Context, appSlug, prompt string, md *meta.Data, proposed []ServiceInput, notifier AINotifier) (string, error) {
	epCache := &endpointCache{
		eps:      make(map[string]*endpointStructs),
		proposed: proposed,
	}
	return query[struct {
		StreamUpdate *StreamUpdate `graphql:"result: defineEndpoints(appSlug: $appSlug, prompt: $prompt, current: $current, proposedDesign: $proposedDesign)"`
	}](ctx, m.client, map[string]interface{}{
		"appSlug":        appSlug,
		"prompt":         prompt,
		"current":        currentService(md),
		"proposedDesign": proposed,
	}, func(ctx context.Context, msg *WSNotification) error {
		switch val := msg.Value.(type) {
		case TypeUpdate:
			ep := epCache.endpoint(val.Service, val.Endpoint)
			ep.upsertStruct(val.Name, val.Doc)
			msg.Value = ep.Notification()
		case TypeFieldUpdate:
			ep := epCache.endpoint(val.Service, val.Endpoint)
			ep.upsertField(val)
			msg.Value = ep.Notification()
		}
		return notifier(ctx, msg)
	})
}

func (m *Manager) ProposeSystemDesign(ctx context.Context, appSlug, prompt string, md *meta.Data, replier AINotifier) (string, error) {
	return query[struct {
		StreamUpdate *StreamUpdate `graphql:"result: proposeSystemDesign(appSlug: $appSlug, prompt: $prompt, current: $current)"`
	}](ctx, m.client, map[string]interface{}{
		"appSlug": appSlug,
		"prompt":  prompt,
		"current": currentService(md),
	}, replier)
}

func (m *Manager) ModifySystemDesign(ctx context.Context, appSlug, originalPrompt string, proposed []ServiceInput, newPrompt string, md *meta.Data, replier AINotifier) (string, error) {
	return query[struct {
		StreamUpdate *StreamUpdate `graphql:"result: modifySystemDesign(appSlug: $appSlug, originalPrompt: $originalPrompt, proposedDesign: $proposedDesign, newPrompt: $newPrompt, current: $current)"`
	}](ctx, m.client, map[string]interface{}{
		"appSlug":        appSlug,
		"originalPrompt": originalPrompt,
		"proposedDesign": fns.Map(proposed, ServiceInput.GraphQL),
		"current":        currentService(md),
		"newPrompt":      newPrompt,
	}, replier)
}

func currentService(md *meta.Data) []ServiceInput {
	var services []ServiceInput
	for _, metaSvc := range md.Svcs {
		svc := ServiceInput{
			Name: metaSvc.Name,
		}
		for _, rpc := range metaSvc.Rpcs {
			ep := &EndpointInput{
				Name:       rpc.Name,
				Method:     rpc.HttpMethods[0],
				Visibility: accessTypeToVisibility(rpc.AccessType),
				Path:       metaPathToPathSegments(rpc.Path),
			}
			if rpc.RequestSchema != nil {
				ep.RequestType = md.Decls[rpc.RequestSchema.GetNamed().Id].Name
			}
			if rpc.ResponseSchema != nil {
				ep.ResponseType = md.Decls[rpc.ResponseSchema.GetNamed().Id].Name
			}
			svc.Endpoints = append(svc.Endpoints, ep)
		}
		services = append(services, svc)
	}
	return services
}

func metaPathToPathSegments(metaPath *meta.Path) []PathSegment {
	var segments []PathSegment
	for _, seg := range metaPath.Segments {
		segments = append(segments, PathSegment{
			Type:      toSegmentType(seg.Type),
			Value:     ptr(seg.Value),
			ValueType: ptr(toSegmentValueType(seg.ValueType)),
		})
	}
	return segments
}

func toSegmentValueType(valueType meta.PathSegment_ParamType) SegmentValueType {
	switch valueType {
	case meta.PathSegment_STRING, meta.PathSegment_UUID:
		return SegmentValueTypeString
	case meta.PathSegment_BOOL:
		return SegmentValueTypeBool
	case meta.PathSegment_INT, meta.PathSegment_INT8, meta.PathSegment_INT16, meta.PathSegment_INT32, meta.PathSegment_INT64,
		meta.PathSegment_UINT, meta.PathSegment_UINT8, meta.PathSegment_UINT16, meta.PathSegment_UINT32, meta.PathSegment_UINT64:
		return SegmentValueTypeInt
	default:
		panic("unknown segment value type")
	}
}

func toSegmentType(segmentType meta.PathSegment_SegmentType) SegmentType {
	switch segmentType {
	case meta.PathSegment_LITERAL:
		return SegmentTypeLiteral
	case meta.PathSegment_PARAM:
		return SegmentTypeParam
	case meta.PathSegment_WILDCARD:
		return SegmentTypeWildcard
	case meta.PathSegment_FALLBACK:
		return SegmentTypeFallback
	default:
		panic("unknown segment type")
	}
}

func accessTypeToVisibility(accessType meta.RPC_AccessType) VisibilityType {
	switch accessType {
	case meta.RPC_PUBLIC:
		return VisibilityTypePublic
	case meta.RPC_PRIVATE:
		return VisibilityTypePrivate
	case meta.RPC_AUTH:
		return ""
	default:
		panic("unknown access type")
	}
}
