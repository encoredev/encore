package ai

import (
	"context"
	"strings"
	"sync"

	"github.com/hasura/go-graphql-client"
	"github.com/hasura/go-graphql-client/pkg/jsonutil"
	"golang.org/x/exp/slices"

	"encr.dev/cli/daemon/apps"
	"encr.dev/pkg/fns"
	"encr.dev/pkg/idents"
	"encr.dev/pkg/paths"
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
	SessionUpdate   `graphql:"... on SessionUpdate"`
	TitleUpdate     `graphql:"... on TitleUpdate"`
	PathParamUpdate `graphql:"... on PathParamUpdate"`
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
	case "SessionUpdate":
		return u.SessionUpdate
	case "TitleUpdate":
		return u.TitleUpdate
	case "PathParamUpdate":
		return u.PathParamUpdate
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

type cachedEndpoint struct {
	service  string
	endpoint *EndpointInput
}

func (e *cachedEndpoint) notification() LocalEndpointUpdate {
	e.endpoint.EndpointSource = e.endpoint.Render()
	e.endpoint.TypeSource = ""
	for i, s := range e.endpoint.Types {
		if i > 0 {
			e.endpoint.TypeSource += "\n"
		}
		e.endpoint.TypeSource += s.Render()
	}
	return LocalEndpointUpdate{
		Service:  e.service,
		Endpoint: e.endpoint,
		Type:     "EndpointUpdate",
	}
}

func (e *cachedEndpoint) upsertType(name, doc string) *TypeInput {
	if name == "" {
		return nil
	}
	for _, s := range e.endpoint.Types {
		if s.Name == name {
			if doc != "" {
				s.Doc = wrapDoc(doc, 77)
			}
			return s
		}
	}
	si := &TypeInput{Name: name, Doc: wrapDoc(doc, 77)}
	e.endpoint.Types = append(e.endpoint.Types, si)
	return si
}

func wrapDoc(doc string, width int) string {
	doc = strings.ReplaceAll(doc, "\n", " ")
	doc = strings.TrimSpace(doc)
	bytes := []byte(doc)
	i := 0
	for {
		start := i
		if start+width >= len(bytes) {
			break
		}
		i += width
		for i > start && bytes[i] != ' ' {
			i--
		}
		if i > start {
			bytes[i] = '\n'
		} else {
			for i < len(bytes) && bytes[i] != ' ' {
				i++
			}
			if i < len(bytes) {
				bytes[i] = '\n'
			}
		}
	}
	return string(bytes)
}

func (e *cachedEndpoint) upsertError(err ErrorUpdate) *ErrorInput {
	for _, s := range e.endpoint.Errors {
		if s.Code == err.Code {
			if err.Doc != "" {
				s.Doc = wrapDoc(err.Doc, 60)
			}
			return s
		}
	}
	si := &ErrorInput{Code: err.Code, Doc: wrapDoc(err.Doc, 60)}
	e.endpoint.Errors = append(e.endpoint.Errors, si)
	return si
}

func (e *cachedEndpoint) upsertPathParam(up PathParamUpdate) PathSegment {
	for i, s := range e.endpoint.Path {
		if s.Value != nil && *s.Value == up.Param {
			if up.Doc != "" {
				e.endpoint.Path[i].Doc = wrapDoc(up.Doc, 73)
			}
			return s
		}
	}
	seg := PathSegment{
		Type:      SegmentTypeParam,
		ValueType: ptr[SegmentValueType]("string"),
		Value:     &up.Param,
		Doc:       wrapDoc(up.Doc, 73),
	}
	e.endpoint.Path = append(e.endpoint.Path, seg)
	return seg
}

func (e *cachedEndpoint) upsertField(up TypeFieldUpdate) *TypeInput {
	if up.Struct == "" {
		return nil
	}
	s := e.upsertType(up.Struct, "")
	for _, f := range s.Fields {
		if f.Name == up.Name {
			if up.Doc != "" {
				f.Doc = wrapDoc(up.Doc, 73)
			}
			if up.Type != "" {
				f.Type = up.Type
			}
			return s
		}
	}
	defaultLoc := apienc.Body
	if slices.Contains([]string{"GET", "HEAD", "DELETE"}, e.endpoint.Method) {
		defaultLoc = apienc.Query
	}
	fi := &TypeFieldInput{
		Name:     up.Name,
		Doc:      wrapDoc(up.Doc, 73),
		Type:     up.Type,
		Location: defaultLoc,
		WireName: idents.Convert(up.Name, idents.CamelCase),
	}
	s.Fields = append(s.Fields, fi)
	return s
}

type endpointCache struct {
	eps      map[string]*cachedEndpoint
	existing []ServiceInput
}

func (s *endpointCache) upsertEndpoint(e EndpointUpdate) *cachedEndpoint {
	for _, ep := range s.eps {
		if ep.service != e.Service || ep.endpoint.Name != e.Name {
			continue
		}
		if e.Doc != "" {
			ep.endpoint.Doc = wrapDoc(e.Doc, 77)
		}
		if e.Method != "" {
			ep.endpoint.Method = e.Method
		}
		if e.Visibility != "" {
			ep.endpoint.Visibility = e.Visibility
		}
		if e.Path != nil {
			ep.endpoint.Path = e.Path
		}
		if e.RequestType != "" {
			ep.endpoint.RequestType = e.RequestType
			ep.upsertType(e.RequestType, "")
		}
		if e.ResponseType != "" {
			ep.endpoint.ResponseType = e.ResponseType
			ep.upsertType(e.ResponseType, "")
		}
		if e.Errors != nil {
			ep.endpoint.Errors = fns.Map(e.Errors, func(e string) *ErrorInput {
				return &ErrorInput{Code: e}
			})
		}
		return ep
	}
	ep := &cachedEndpoint{
		service: e.Service,
		endpoint: &EndpointInput{
			Name:         e.Name,
			Doc:          wrapDoc(e.Doc, 77),
			Method:       e.Method,
			Visibility:   e.Visibility,
			Path:         e.Path,
			RequestType:  e.RequestType,
			ResponseType: e.ResponseType,
			Errors: fns.Map(e.Errors, func(e string) *ErrorInput {
				return &ErrorInput{Code: e}
			}),
			Language: "GO",
		},
	}
	for _, t := range []string{e.RequestType, e.ResponseType} {
		if t == "" {
			continue
		}
		ep.endpoint.Types = append(ep.endpoint.Types, &TypeInput{Name: t})
	}
	return ep
}

func (s *endpointCache) endpoint(service, endpoint string) *cachedEndpoint {
	key := service + "." + endpoint
	if _, ok := s.eps[key]; !ok {
		for _, svc := range s.existing {
			if svc.Name != service {
				continue
			}
			for _, ep := range svc.Endpoints {
				if ep.Name != endpoint {
					continue
				}
				s.eps[key] = &cachedEndpoint{
					service:  service,
					endpoint: ep,
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

func createUpdateHandler(existing []ServiceInput, notifier AINotifier) AINotifier {
	epCache := &endpointCache{
		eps:      make(map[string]*cachedEndpoint),
		existing: existing,
	}
	var lastEp *cachedEndpoint
	return func(ctx context.Context, msg *WSNotification) error {
		var ep *cachedEndpoint
		msgVal := msg.Value
		switch val := msg.Value.(type) {
		case TypeUpdate:
			ep = epCache.endpoint(val.Service, val.Endpoint)
			ep.upsertType(val.Name, val.Doc)
			msgVal = ep.notification()
		case TypeFieldUpdate:
			ep = epCache.endpoint(val.Service, val.Endpoint)
			ep.upsertField(val)
			msgVal = ep.notification()
		case EndpointUpdate:
			ep = epCache.upsertEndpoint(val)
			msgVal = ep.notification()
		case ErrorUpdate:
			ep = epCache.endpoint(val.Service, val.Endpoint)
			ep.upsertError(val)
			msgVal = ep.notification()
		case PathParamUpdate:
			ep = epCache.endpoint(val.Service, val.Endpoint)
			ep.upsertPathParam(val)
			msgVal = ep.notification()
		}
		if lastEp != ep {
			if lastEp != nil {
				msg.Value = struct {
					Type     string `json:"type"`
					Service  string `json:"service"`
					Endpoint string `json:"endpoint"`
				}{"EndpointComplete", lastEp.service, lastEp.endpoint.Name}
				if err := notifier(ctx, msg); err != nil || msg.Finished {
					return err
				}
			}
			lastEp = ep
		}
		msg.Value = msgVal
		return notifier(ctx, msg)
	}
}

func (m *Manager) DefineEndpoints(ctx context.Context, appSlug string, sessionID AISessionID, prompt string, md *meta.Data, proposed []ServiceInput, notifier AINotifier) (string, error) {
	return query[struct {
		StreamUpdate *StreamUpdate `graphql:"result: defineEndpoints(appSlug: $appSlug, sessionID: $sessionID, prompt: $prompt, current: $current, proposedDesign: $proposedDesign)"`
	}](ctx, m.client, map[string]interface{}{
		"appSlug":        appSlug,
		"prompt":         prompt,
		"current":        currentService(md),
		"proposedDesign": fns.Map(proposed, ServiceInput.GraphQL),
		"sessionID":      sessionID,
	}, createUpdateHandler(proposed, notifier))
}

func (m *Manager) ProposeSystemDesign(ctx context.Context, appSlug, prompt string, md *meta.Data, notifier AINotifier) (string, error) {
	return query[struct {
		StreamUpdate *StreamUpdate `graphql:"result: proposeSystemDesign(appSlug: $appSlug, prompt: $prompt, current: $current)"`
	}](ctx, m.client, map[string]interface{}{
		"appSlug": appSlug,
		"prompt":  prompt,
		"current": currentService(md),
	}, createUpdateHandler(nil, notifier))
}

func (m *Manager) ModifySystemDesign(ctx context.Context, appSlug string, sessionID AISessionID, originalPrompt string, proposed []ServiceInput, newPrompt string, md *meta.Data, notifier AINotifier) (string, error) {
	return query[struct {
		StreamUpdate *StreamUpdate `graphql:"result: modifySystemDesign(appSlug: $appSlug, sessionID: $sessionID, originalPrompt: $originalPrompt, proposedDesign: $proposedDesign, newPrompt: $newPrompt, current: $current)"`
	}](ctx, m.client, map[string]interface{}{
		"appSlug":        appSlug,
		"originalPrompt": originalPrompt,
		"proposedDesign": fns.Map(proposed, ServiceInput.GraphQL),
		"current":        currentService(md),
		"newPrompt":      newPrompt,
		"sessionID":      sessionID,
	}, createUpdateHandler(proposed, notifier))
}

func ParseCode(ctx context.Context, services []ServiceInput, app *apps.Instance) (*SyncResult, error) {
	return parseCode(ctx, app, services)
}

func UpdateCode(ctx context.Context, services []ServiceInput, app *apps.Instance, overwrite bool) (*SyncResult, error) {
	return updateCode(ctx, services, app, overwrite)
}

type WriteFilesResponse struct {
	FilesPaths []paths.RelSlash `json:"paths"`
}

func WriteFiles(ctx context.Context, services []ServiceInput, app *apps.Instance) (*WriteFilesResponse, error) {
	files, err := writeFiles(services, app)
	return &WriteFilesResponse{FilesPaths: files}, err
}

type PreviewFile struct {
	Path    paths.RelSlash `json:"path"`
	Content string         `json:"content"`
}

type PreviewFilesResponse struct {
	Files []PreviewFile `json:"files"`
}

func PreviewFiles(ctx context.Context, services []ServiceInput, app *apps.Instance) (*PreviewFilesResponse, error) {
	files, err := generateSrcFiles(services, app)
	return &PreviewFilesResponse{Files: fns.TransformMapToSlice(files, func(k paths.RelSlash, v string) PreviewFile {
		return PreviewFile{Path: k, Content: v}
	})}, err
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
