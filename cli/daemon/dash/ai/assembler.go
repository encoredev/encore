package ai

import (
	"context"
	"slices"
	"strings"

	"encr.dev/pkg/fns"
	"encr.dev/pkg/idents"
	"encr.dev/v2/parser/apis/api/apienc"
)

// partialEndpoint is a helper struct that is used to assemble the endpoint
// from the incoming websocket updates.
type partialEndpoint struct {
	service  string
	endpoint *Endpoint
}

// notification generates a partially assembled endpoint structure to return to the client
func (e *partialEndpoint) notification() LocalEndpointUpdate {
	e.endpoint.EndpointSource = e.endpoint.Render()
	e.endpoint.TypeSource = ""
	for i, s := range e.endpoint.Types {
		if i > 0 {
			e.endpoint.TypeSource += "\n\n"
		}
		e.endpoint.TypeSource += s.Render()
	}
	return LocalEndpointUpdate{
		Service:  e.service,
		Endpoint: e.endpoint,
		Type:     "EndpointUpdate",
	}
}

func (e *partialEndpoint) upsertType(name, doc string) *Type {
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
	si := &Type{Name: name, Doc: wrapDoc(doc, 77)}
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

func (e *partialEndpoint) upsertError(err ErrorUpdate) *Error {
	for _, s := range e.endpoint.Errors {
		if s.Code == err.Code {
			if err.Doc != "" {
				s.Doc = wrapDoc(err.Doc, 60)
			}
			return s
		}
	}
	si := &Error{Code: err.Code, Doc: wrapDoc(err.Doc, 60)}
	e.endpoint.Errors = append(e.endpoint.Errors, si)
	return si
}

func (e *partialEndpoint) upsertPathParam(up PathParamUpdate) PathSegment {
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

func (e *partialEndpoint) upsertField(up TypeFieldUpdate) *Type {
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
	isRequest := up.Struct == e.endpoint.RequestType
	if slices.Contains([]string{"GET", "HEAD", "DELETE"}, e.endpoint.Method) && isRequest {
		defaultLoc = apienc.Query
	}
	fi := &TypeField{
		Name:     up.Name,
		Doc:      wrapDoc(up.Doc, 73),
		Type:     up.Type,
		Location: defaultLoc,
		WireName: idents.Convert(up.Name, idents.CamelCase),
	}
	s.Fields = append(s.Fields, fi)
	return s
}

// The endpointsAssembler is a helper struct that is used to assemble the endpoint
// from the incoming websocket updates. It keeps track of the existing endpoints and services
// and updates them accordingly.
type endpointsAssembler struct {
	eps      map[string]*partialEndpoint
	existing []Service
}

func (s *endpointsAssembler) upsertEndpoint(e EndpointUpdate) *partialEndpoint {
	s.endpoint(e.Service, e.Name)
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
		if len(e.Path) > 0 {
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
			ep.endpoint.Errors = fns.Map(e.Errors, func(e string) *Error {
				return &Error{Code: e}
			})
		}
		return ep
	}
	ep := &partialEndpoint{
		service: e.Service,
		endpoint: &Endpoint{
			Name:         e.Name,
			Doc:          wrapDoc(e.Doc, 77),
			Method:       e.Method,
			Visibility:   e.Visibility,
			Path:         e.Path,
			RequestType:  e.RequestType,
			ResponseType: e.ResponseType,
			Errors: fns.Map(e.Errors, func(e string) *Error {
				return &Error{Code: e}
			}),
			Language: "GO",
		},
	}
	s.eps[e.Service+"."+e.Name] = ep
	return ep
}

func (s *endpointsAssembler) endpoint(service, endpoint string) *partialEndpoint {
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
				s.eps[key] = &partialEndpoint{
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

func newEndpointAssemblerHandler(existing []Service, notifier AINotifier, epComplete bool) AINotifier {
	epCache := &endpointsAssembler{
		eps:      make(map[string]*partialEndpoint),
		existing: existing,
	}
	var lastEp *partialEndpoint
	return func(ctx context.Context, msg *AINotification) error {
		var ep *partialEndpoint
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
		if epComplete && lastEp != ep {
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
