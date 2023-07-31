package daemon

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"

	"encr.dev/cli/daemon/apps"
	"encr.dev/cli/daemon/namespace"
	"encr.dev/pkg/fns"
	daemonpb "encr.dev/proto/encore/daemon"
)

func (s *Server) CreateNamespace(ctx context.Context, req *daemonpb.CreateNamespaceRequest) (*daemonpb.Namespace, error) {
	app, err := s.apps.Track(req.AppRoot)
	if err != nil {
		return nil, err
	}
	ns, err := s.ns.Create(ctx, app, namespace.Name(req.Name))
	if err != nil {
		return nil, err
	}
	return ns.ToProto(), nil
}

func (s *Server) ListNamespaces(ctx context.Context, req *daemonpb.ListNamespacesRequest) (*daemonpb.ListNamespacesResponse, error) {
	app, err := s.apps.Track(req.AppRoot)
	if err != nil {
		return nil, err
	}
	nss, err := s.ns.List(ctx, app)
	if err != nil {
		return nil, err
	}
	protos := fns.Map(nss, (*namespace.Namespace).ToProto)
	return &daemonpb.ListNamespacesResponse{Namespaces: protos}, nil
}

func (s *Server) DeleteNamespace(ctx context.Context, req *daemonpb.DeleteNamespaceRequest) (*empty.Empty, error) {
	app, err := s.apps.Track(req.AppRoot)
	if err != nil {
		return nil, err
	}
	if err := s.ns.Delete(ctx, app, namespace.Name(req.Name)); err != nil {
		return nil, err
	}
	return &empty.Empty{}, nil
}

func (s *Server) SwitchNamespace(ctx context.Context, req *daemonpb.SwitchNamespaceRequest) (*daemonpb.Namespace, error) {
	app, err := s.apps.Track(req.AppRoot)
	if err != nil {
		return nil, err
	}

	if req.Create {
		_, err := s.ns.Create(ctx, app, namespace.Name(req.Name))
		if err != nil {
			return nil, err
		}
	}

	ns, err := s.ns.Switch(ctx, app, namespace.Name(req.Name))
	if err != nil {
		return nil, err
	}
	return ns.ToProto(), nil
}

func (s *Server) namespaceOrActive(ctx context.Context, app *apps.Instance, ns *string) (*namespace.Namespace, error) {
	if ns == nil {
		return s.ns.GetActive(ctx, app)
	}
	return s.ns.GetByName(ctx, app, namespace.Name(*ns))
}
