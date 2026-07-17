package api

import (
	"net/http"
	"testing"

	"github.com/benbjohnson/clock"
	"github.com/frankban/quicktest"

	"encore.dev/appruntime/apisdk/api/svcauth"
	"encore.dev/appruntime/apisdk/api/transport"
	"encore.dev/appruntime/exported/config"
	"encore.dev/appruntime/exported/model"
)

// TestGatewayInboundMetaNotPropagated checks that Encore metadata supplied on an
// external (non-internal) request is not propagated to the upstream service when
// the gateway proxies it. The gateway re-derives the metadata itself, so inbound
// values (such as the user id) must not survive to the backend.
func TestGatewayInboundMetaNotPropagated(t *testing.T) {
	q := quicktest.New(t)
	klock := clock.NewMock()

	const targetSvc = "target"
	rtCfg := &config.Runtime{
		AppSlug:     "app",
		EnvName:     "env",
		EnvCloud:    "local",
		AuthKeys:    []config.EncoreAuthKey{{KeyID: 1, Data: []byte("0123456789abcdef0123456789abcdef")}},
		ServiceAuth: []config.ServiceAuth{{Method: "encore-auth"}},
		ServiceDiscovery: map[string]config.Service{
			targetSvc: {
				Name:        targetSvc,
				URL:         "http://localhost:0",
				Protocol:    config.Http,
				ServiceAuth: config.ServiceAuth{Method: "encore-auth"},
			},
		},
	}

	inbound, outbound, err := svcauth.LoadMethods(klock, rtCfg)
	q.Assert(err, quicktest.IsNil)

	srv := &Server{
		runtime:         rtCfg,
		clock:           klock,
		inboundSvcAuth:  inbound,
		outboundSvcAuth: outbound,
	}

	// An external request that carries inbound Encore metadata.
	inboundReq, err := http.NewRequest("GET", "http://gateway/whoami", nil)
	q.Assert(err, quicktest.IsNil)
	inboundReq.Header.Set("X-Encore-Meta-UserID", "admin")

	// The reverse proxy clones the inbound request onto the upstream request.
	upstream := inboundReq.Clone(inboundReq.Context())

	// The request is not from a verified internal caller, so the gateway drops
	// inbound metadata before signing and forwarding it.
	stripInboundMeta(upstream.Header)

	traceID, _ := model.GenTraceID()
	gwMeta := CallMeta{
		TraceID: traceID,
		Internal: &InternalCallMeta{
			Caller: GatewayCaller{GatewayName: "api-gateway"},
		},
	}
	err = gwMeta.AddToRequest(srv, rtCfg.ServiceDiscovery[targetSvc], transport.HTTPRequest(upstream))
	q.Assert(err, quicktest.IsNil)

	svcMeta, err := srv.MetaFromRequest(transport.HTTPRequest(upstream))
	q.Assert(err, quicktest.IsNil)

	gotUID := ""
	if svcMeta.Internal != nil {
		gotUID = svcMeta.Internal.AuthUID
	}
	q.Assert(gotUID, quicktest.Equals, "")
}
