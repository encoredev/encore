package rtconfgen

import (
	"fmt"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/golang/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"encr.dev/pkg/fns"
	"encr.dev/pkg/option"
	meta "encr.dev/proto/encore/parser/meta/v1"
	runtimev1 "encr.dev/proto/encore/runtime/v1"
)

type ResourceID interface {
	comparable
	fmt.Stringer
}

type Builder struct {
	Infra *InfraBuilder
	rs    *resourceSet

	env    *runtimev1.Environment
	encore *runtimev1.EncorePlatform
	obs    *runtimev1.Observability

	// Any errors encountered during the build process.
	err error

	// authMethods are the service auth methods to use for authenticating to this deployment.
	authMethods []*runtimev1.ServiceAuth

	// defaultGracefulShutdown is the graceful shutdown behavior to use by default,
	// unless a deployment overrides it.
	defaultGracefulShutdown *runtimev1.GracefulShutdown

	defaultDeployID   string
	defaultDeployedAt time.Time

	deployments map[string]*Deployment
}

func NewBuilder() *Builder {
	rs := new(resourceSet)
	b := &Builder{
		Infra:       newInfraBuilder(rs),
		rs:          rs,
		obs:         &runtimev1.Observability{},
		deployments: make(map[string]*Deployment),
	}

	return b
}

func (b *Builder) Env(env *runtimev1.Environment) *Builder {
	b.env = env
	return b
}

func (b *Builder) EncorePlatform(encore *runtimev1.EncorePlatform) *Builder {
	b.encore = encore
	return b
}

func (b *Builder) DefaultGracefulShutdown(s *runtimev1.GracefulShutdown) *Builder {
	b.defaultGracefulShutdown = s
	return b
}

func (b *Builder) AuthMethods(m []*runtimev1.ServiceAuth) *Builder {
	b.authMethods = m
	return b
}

func (b *Builder) DeployID(id string) *Builder {
	b.defaultDeployID = id
	return b
}

func (b *Builder) DeployedAt(t time.Time) *Builder {
	b.defaultDeployedAt = t
	return b
}

func (b *Builder) TracingProvider(p *runtimev1.TracingProvider) {
	b.TracingProviderFn(p.Rid, tofn(p))
}

func (b *Builder) TracingProviderFn(rid string, fn func() *runtimev1.TracingProvider) {
	addResFunc(&b.obs.Tracing, b.rs, rid, fn)
}

func (b *Builder) MetricsProvider(p *runtimev1.MetricsProvider) {
	b.MetricsProviderFn(p.Rid, tofn(p))
}

func (b *Builder) MetricsProviderFn(rid string, fn func() *runtimev1.MetricsProvider) {
	addResFunc(&b.obs.Metrics, b.rs, rid, fn)
}

func (b *Builder) LogsProvider(p *runtimev1.LogsProvider) {
	b.LogsProviderFn(p.Rid, tofn(p))
}

func (b *Builder) LogsProviderFn(rid string, fn func() *runtimev1.LogsProvider) {
	addResFunc(&b.obs.Logs, b.rs, rid, fn)
}

func (b *Builder) Deployment(rid string) *Deployment {
	if d, ok := b.deployments[rid]; ok {
		return d
	}
	d := &Deployment{b: b, rid: rid}
	b.deployments[rid] = d
	return d
}

type Deployment struct {
	b   *Builder
	rid string

	deployID   option.Option[string]
	deployedAt option.Option[time.Time]
	reduceWith option.Option[*meta.Data]

	dynamicExperiments []string

	// The graceful shutdown behavior to use for this deployment.
	// If None, uses the default graceful shutdown behavior.
	gracefulShutdown option.Option[*runtimev1.GracefulShutdown]

	// The service-discovery configuration for this deployment.
	sd *runtimev1.ServiceDiscovery

	// The base URL for reaching this deployment from another service.
	svc2svcBaseURL string

	hostedGateways []string
	hostedServices []string
}

// DeployID sets the deploy id.
func (d *Deployment) DeployID(id string) *Deployment {
	d.deployID = option.Some(id)
	return d
}

// OverrideDeployedAt sets the time of deploy.
func (d *Deployment) OverrideDeployedAt(t time.Time) *Deployment {
	d.deployedAt = option.Some(t)
	return d
}

func (d *Deployment) DynamicExperiments(experiments []string) *Deployment {
	d.dynamicExperiments = experiments
	return d
}

// HostsServices adds the given service names as being hosted by this deployment.
// It appends and doesn't overwrite any existing hosted services.
func (d *Deployment) HostsServices(names ...string) *Deployment {
	d.hostedServices = append(d.hostedServices, names...)
	return d
}

// HostsGateways adds the given gateway names as being hosted by this deployment.
// It appends and doesn't overwrite any existing hosted gateways.
func (d *Deployment) HostsGateways(names ...string) *Deployment {
	d.hostedGateways = append(d.hostedGateways, names...)
	return d
}

// OverrideGracefulShutdown sets the graceful shutdown behavior for this specific deployment.
// To set a default graceful shutdown shared for all deployments, use [Builder.DefaultGracefulShutdown] instead.
func (d *Deployment) OverrideGracefulShutdown(s *runtimev1.GracefulShutdown) *Deployment {
	d.gracefulShutdown = option.Some(s)
	return d
}

func (d *Deployment) ServiceDiscovery(sd *runtimev1.ServiceDiscovery) *Deployment {
	d.sd = sd
	return d
}

func (d *Deployment) ReduceWithMeta(md *meta.Data) *Deployment {
	d.reduceWith = option.Some(md)
	return d
}

func (d *Deployment) BuildRuntimeConfig() (*runtimev1.RuntimeConfig, error) {
	b := d.b

	infra, err := b.Infra.get()
	if err != nil {
		return nil, err
	}
	if reduced, ok := d.reduceWith.Get(); ok {
		infra = reduceForServices(infra, reduced, d.hostedServices)
	}

	graceful := d.gracefulShutdown.GetOrElse(d.b.defaultGracefulShutdown)

	var hostedServices []*runtimev1.HostedService
	for _, svcName := range d.hostedServices {
		hostedServices = append(hostedServices, &runtimev1.HostedService{
			Name: svcName,
		})
	}

	gatewaysByName := make(map[string]*runtimev1.Gateway)
	for _, gw := range infra.Resources.Gateways {
		gatewaysByName[gw.EncoreName] = gw
	}

	gatewayRids := fns.Map(d.hostedGateways, func(name string) string {
		gw, ok := gatewaysByName[name]
		if !ok {
			b.setErrf("gateway %q not found", name)
			return ""
		}
		return gw.Rid
	})

	deploy := &runtimev1.Deployment{
		HostedGateways:     gatewayRids,
		HostedServices:     hostedServices,
		ServiceDiscovery:   d.sd,
		GracefulShutdown:   graceful,
		DynamicExperiments: d.dynamicExperiments,
		Observability:      b.obs,
		AuthMethods:        d.b.authMethods,
		DeployId:           d.deployID.GetOrElse(b.defaultDeployID),
		DeployedAt:         timestamppb.New(d.deployedAt.GetOrElse(b.defaultDeployedAt)),
	}

	cfg := &runtimev1.RuntimeConfig{
		Environment:    b.env,
		EncorePlatform: b.encore,
		Infra:          infra,
		Deployment:     deploy,
	}

	// Deep-clone the protobuf to avoid subsequent mutations from modifying this one.
	cfg = cloneProto(cfg)

	return cfg, b.err
}

func (b *Builder) setErr(err error) {
	if b.err == nil {
		b.err = err
	}
}

func (b *Builder) setErrf(format string, args ...any) {
	b.setErr(errors.Newf(format, args...))
}

func cloneProto[M proto.Message](m M) M {
	return proto.Clone(m).(M)
}
