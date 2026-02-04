package export

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"

	"encr.dev/internal/conf"
	"encr.dev/internal/urlutil"

	"github.com/cockroachdb/errors"
	"github.com/logrusorgru/aurora"
	"github.com/tailscale/hujson"
	"golang.org/x/exp/maps"

	"encore.dev/appruntime/exported/config/infra"
	"encr.dev/pkg/appfile"
	"encr.dev/pkg/dockerbuild"
	"encr.dev/pkg/fns"
	meta "encr.dev/proto/encore/parser/meta/v1"
)

var (
	LEARN_MORE = aurora.Italic("Learn More: " + urlutil.JoinURL(conf.DocsBaseURL(), "/docs/how-to/self-host")).String()
)

// defaultInfraConfigPath is the path in the image where the environment configuration is mounted.
const defaultInfraConfigPath dockerbuild.ImagePath = "/encore/infra.config.json"

type EmbeddedInfraConfigParams struct {
	// The path to the infra config file.
	File dockerbuild.HostPath

	// Services to include in the image.
	Services []string

	// Gateways to include in the image.
	Gateways []string

	// CORS config to include in the image.
	GlobalCORS appfile.CORS

	Meta *meta.Data
}

func buildAndValidateInfraConfig(params EmbeddedInfraConfigParams) (*infra.InfraConfig, string, error) {
	missing := map[string][]string{}
	md := params.Meta
	services := params.Services
	gateways := params.Gateways
	if len(services)+len(gateways) == 0 {
		services = fns.Map(md.Svcs, (*meta.Service).GetName)
		gateways = fns.Map(md.Gateways, (*meta.Gateway).GetEncoreName)
	}

	unknownServices := fns.Filter(services, func(s string) bool {
		return !fns.Any(md.Svcs, func(svc *meta.Service) bool {
			return svc.Name == s
		})
	})
	if len(unknownServices) > 0 {
		return nil, "", errors.Newf("unknown services: %v", unknownServices)
	}

	unknownGateways := fns.Filter(gateways, func(s string) bool {
		return !fns.Any(md.Gateways, func(gw *meta.Gateway) bool {
			return gw.EncoreName == s
		})
	})
	if len(unknownGateways) > 0 {
		return nil, "", errors.Newf("unknown gateways: %v", unknownGateways)
	}

	var infraCfg infra.InfraConfig
	if params.File != "" {
		data, err := os.ReadFile(params.File.String())
		if err != nil {
			return nil, "", errors.Wrap(err, "infra config not found")
		}
		data, err = hujson.Standardize(data)
		if err != nil {
			return nil, "", errors.Wrap(err, "could not standardize infra config")
		}
		err = json.Unmarshal(data, &infraCfg)
		if err != nil {
			return nil, "", errors.Wrap(err, "could not decode infra config")
		}
	}
	infraCfg.HostedGateways = gateways
	infraCfg.HostedServices = services
	envVars, validationErrors := infra.Validate(&infraCfg)

	hostedSvcs := fns.ToMap(fns.Filter(md.Svcs, func(svc *meta.Service) bool {
		return fns.Any(services, func(s string) bool {
			return svc.Name == s
		})
	}), (*meta.Service).GetName)

	var secrets []string
	// Find all service dependencies for our hosted services.
	var svcDeps = map[string]struct{}{}
	pkgs := fns.ToMap(md.Pkgs, (*meta.Package).GetRelPath)

	// Add dependencies for all outbound RPCs for our hosted services
	// and collect all required secrets.
	for _, p := range md.Pkgs {
		if p.ServiceName == "" {
			secrets = append(secrets, p.Secrets...)
			continue
		} else if _, ok := hostedSvcs[p.ServiceName]; !ok {
			continue
		}
		secrets = append(secrets, p.Secrets...)
		for _, r := range p.RpcCalls {
			svcDeps[pkgs[r.Pkg].ServiceName] = struct{}{}
		}
	}

	// Add auth handler to service discovery if we host any auth RPCs.
	if md.AuthHandler != nil {
		requiresAuth := fns.Any(md.Svcs, func(svc *meta.Service) bool {
			return fns.Any(svc.Rpcs, func(rpc *meta.RPC) bool {
				return rpc.AccessType == meta.RPC_AUTH
			})
		})
		if requiresAuth {
			svcDeps[md.AuthHandler.ServiceName] = struct{}{}
		}
	}

	// Make sure we have service discovery for all services that are not private
	// if we are hosting gateways.
	if len(gateways) > 0 {
		for _, svc := range md.Svcs {
			if _, ok := hostedSvcs[svc.Name]; ok {
				continue
			}
			for _, rpc := range svc.Rpcs {
				if rpc.AccessType != meta.RPC_PRIVATE {
					svcDeps[svc.Name] = struct{}{}
					break
				}
			}
		}
	}

	// Remove any services that we host from our service dependencies.
	for _, svc := range hostedSvcs {
		delete(svcDeps, svc.Name)
	}

	// Remove any service discovery entries for services that we don't host.
	for svc := range infraCfg.ServiceDiscovery {
		if _, ok := svcDeps[svc]; !ok {
			delete(infraCfg.ServiceDiscovery, svc)
		} else {
			delete(svcDeps, svc)
		}
	}

	// Make sure all our service dependencies are accounted for.
	if len(svcDeps) > 0 {
		missing["Service Discovery"] = maps.Keys(svcDeps)
	}

	// Remove secrets we don't need for our hosted services.
	slices.Sort(secrets)
	secrets = slices.Compact(secrets)
	var ok bool
	if infraCfg.Secrets.EnvRef == nil {
		for secret := range infraCfg.Secrets.SecretsMap {
			secrets, ok = fns.Delete(secrets, secret)
			if !ok {
				delete(infraCfg.Secrets.SecretsMap, secret)
			}
		}

		// Make sure all our secrets are accounted for.
		if len(secrets) > 0 {
			missing["Secrets"] = secrets
		}
	} else {
		// Print that you need to define a secrets map in the infra config.
	}

	// Find all databases for our hosted services.
	databases := fns.FlatMap(maps.Values(hostedSvcs), func(db *meta.Service) []string {
		return db.Databases
	})
	slices.Sort(databases)
	databases = slices.Compact(databases)

	for i, sqlServer := range append([]*infra.SQLServer{}, infraCfg.SQLServers...) {
		for name := range sqlServer.Databases {
			databases, ok = fns.Delete(databases, name)
			if !ok {
				delete(sqlServer.Databases, name)
			}
		}
		if len(sqlServer.Databases) == 0 {
			infraCfg.SQLServers = append(infraCfg.SQLServers[:i], infraCfg.SQLServers[i+1:]...)
		}
	}

	if len(databases) > 0 {
		missing["Databases"] = databases
	}

	caches := fns.MapAndFilter(md.CacheClusters, func(cache *meta.CacheCluster) (string, bool) {
		return cache.Name, fns.Any(cache.Keyspaces, func(ks *meta.CacheCluster_Keyspace) bool {
			return fns.Any(services, func(s string) bool {
				return ks.Service == s
			})
		})
	})

	for name := range infraCfg.Redis {
		caches, ok = fns.Delete(caches, name)
		if !ok {
			delete(infraCfg.Redis, name)
		}
	}

	if len(caches) > 0 {
		missing["Redis"] = caches
	}

	subscriptions := fns.FlatMap(md.PubsubTopics, func(topic *meta.PubSubTopic) [][2]string {
		return fns.MapAndFilter(topic.Subscriptions, func(s *meta.PubSubTopic_Subscription) ([2]string, bool) {
			return [2]string{topic.Name, s.Name}, fns.Any(services, func(svc string) bool {
				return s.ServiceName == svc
			})
		})
	})

	for _, pubsub := range infraCfg.PubSub {
		for topicName, topic := range pubsub.GetTopics() {
			for subName := range topic.GetSubscriptions() {
				found := false
				for i, sub := range subscriptions {
					if sub[0] == topicName && sub[1] == subName {
						subscriptions = append(subscriptions[:i], subscriptions[i+1:]...)
						found = true
						break
					}
				}
				if !found {
					topic.DeleteSubscription(subName)
				}
			}
		}
	}

	if len(subscriptions) > 0 {
		missing["Subscriptions"] = fns.Map(subscriptions, func(sub [2]string) string {
			return sub[0] + "/" + sub[1]
		})
	}

	topics := fns.MapAndFilter(md.PubsubTopics, func(topic *meta.PubSubTopic) (string, bool) {
		return topic.Name, fns.Any(topic.Publishers, func(p *meta.PubSubTopic_Publisher) bool {
			return fns.Any(services, func(s string) bool {
				return p.ServiceName == s
			})
		})
	})

	for i, pubsub := range infraCfg.PubSub {
		for topicName, topic := range pubsub.GetTopics() {
			i := slices.Index(topics, topicName)
			if i != -1 {
				topics = append(topics[:i], topics[i+1:]...)
			} else if len(topic.GetSubscriptions()) == 0 {
				pubsub.DeleteTopic(topicName)
			}
		}
		if len(pubsub.GetTopics()) == 0 {
			infraCfg.PubSub = append(infraCfg.PubSub[:i], infraCfg.PubSub[i+1:]...)
		}
	}
	if len(topics) > 0 {
		missing["Topics"] = topics
	}

	// Validate bucket config
	buckets := fns.FlatMap(maps.Values(hostedSvcs), func(svc *meta.Service) []string {
		return fns.Map(svc.Buckets, (*meta.BucketUsage).GetBucket)
	})
	slices.Sort(buckets)
	buckets = slices.Compact(buckets)

	for _, storage := range infraCfg.ObjectStorage {
		for name, infraCfg := range storage.GetBuckets() {
			metaBkt, ok := fns.Find(md.Buckets, func(b *meta.Bucket) bool {
				return b.Name == name
			})
			if ok {
				if metaBkt.Public && infraCfg.PublicBaseURL == "" {
					path := infra.JSONPath("buckets").Append(infra.JSONPath(name)).Append("public_base_url")
					validationErrors[path] = errors.New("Bucket is public but no public base URL is set")
					return nil, "", configError(missing, validationErrors)
				}
			}

			buckets, ok = fns.Delete(buckets, name)
			if !ok {
				storage.DeleteBucket(name)
			}
		}
	}
	if len(buckets) > 0 {
		missing["Buckets"] = buckets
	}

	// Copy CORS config
	cors := infra.CORS(params.GlobalCORS)
	infraCfg.CORS = &cors

	if len(missing) > 0 || len(validationErrors) > 0 {
		return nil, "", configError(missing, validationErrors)
	}

	cronJobStr, err := formatCronJobInstructions(services, md)
	if err != nil {
		return nil, "", err
	}
	envStr := formatEnvVars(envVars)
	var resp strings.Builder
	if len(cronJobStr)+len(envStr) > 0 {
		resp.WriteString(aurora.Bold("Before you deploy, you may need to configure the following:\n").String())
		resp.WriteString(cronJobStr)
		resp.WriteString(envStr)
	}
	resp.WriteString(LEARN_MORE)

	return &infraCfg, resp.String(), nil
}

func formatCronJobInstructions(services []string, md *meta.Data) (string, error) {
	if len(md.CronJobs) == 0 {
		return "", nil
	}
	svcByRelPath := fns.ToMap(md.Svcs, func(p *meta.Service) string {
		return p.RelPath
	})
	cronsTable := [][]string{
		{"ID", "Endpoint Path", "Schedule"},
	}
	for _, cronJob := range md.CronJobs {
		svc, ok := svcByRelPath[cronJob.Endpoint.Pkg]
		if !ok {
			return "", errors.Newf("could not find service for cron job %s", cronJob.Id)
		}
		if !slices.Contains(services, svc.Name) {
			continue
		}
		rpc, ok := fns.Find(svc.Rpcs, func(r *meta.RPC) bool {
			return r.Name == cronJob.Endpoint.Name
		})
		if !ok {
			return "", errors.Newf("could not find rpc for cron job %s", cronJob.Id)
		}
		cronsTable = append(cronsTable, []string{cronJob.Id, pathToString(rpc.Path), cronJob.Schedule})
	}
	if len(cronsTable) == 1 {
		return "", nil
	}

	return aurora.Sprintf("\n%s\n%s\n", aurora.Bold("Cron Jobs:"), generateTable(cronsTable)), nil
}

func generateTable(rows [][]string) string {
	au := aurora.NewAurora(true)
	var sb strings.Builder

	// Calculate column widths
	colWidths := make([]int, len(rows[0]))
	for _, row := range rows {
		for i, cell := range row {
			colWidths[i] = max(colWidths[i], len(cell))
		}
	}

	// Helper function to create a horizontal line
	createLine := func() string {
		line := "+"
		for _, width := range colWidths {
			line += strings.Repeat("-", width+2) + "+"
		}
		return line + "\n"
	}

	// Write top border
	sb.WriteString(au.Cyan(createLine()).String())

	// Write header
	sb.WriteString(au.Cyan("| ").String())
	for i, header := range rows[0] {
		sb.WriteString(au.Bold(fmt.Sprintf("%-*s", colWidths[i], header)).String())
		sb.WriteString(au.Cyan(" | ").String())
	}
	sb.WriteString("\n")

	// Write header-content separator
	sb.WriteString(au.Cyan(createLine()).String())

	// Write content rows
	for _, row := range rows[1:] {
		sb.WriteString(au.Cyan("| ").String())
		for i, cell := range row {
			sb.WriteString(fmt.Sprintf("%-*s", colWidths[i], cell))
			sb.WriteString(au.Cyan(" | ").String())
		}
		sb.WriteString("\n")
	}

	// Write bottom border
	sb.WriteString(au.Cyan(createLine()).String())

	return sb.String()
}

func pathToString(path *meta.Path) string {
	b := strings.Builder{}
	for _, s := range path.Segments {
		b.WriteByte('/')
		switch s.Type {
		case meta.PathSegment_PARAM:
			b.WriteByte(':')
		case meta.PathSegment_WILDCARD:
			b.WriteByte('*')
		case meta.PathSegment_FALLBACK:
			b.WriteByte('!')
		}
		b.WriteString(s.Value)
	}
	return b.String()

}

func formatEnvVars(envVars map[infra.JSONPath]infra.EnvDesc) string {
	if len(envVars) == 0 {
		return ""
	}

	envByName := map[string]infra.EnvDesc{}
	for _, envVar := range envVars {
		envByName[envVar.Name] = envVar
	}
	envTable := [][]string{
		{"Name", "Description"},
	}
	for _, envVar := range envByName {
		envTable = append(envTable, []string{envVar.Name, envVar.Description})
	}
	return aurora.Sprintf("%s\n%s\n", aurora.Bold("Environment Variables:"), generateTable(envTable))
}

func configError(missing map[string][]string, validation map[infra.JSONPath]error) error {
	au := aurora.NewAurora(true)
	var errorMsg strings.Builder

	errorMsg.WriteString("\n")
	errorMsg.WriteString(au.Red("\nYour infra configuration is incomplete\n").String())
	errorMsg.WriteString("\n")

	if len(missing) > 0 {
		errorMsg.WriteString(au.Red("Missing Resource Configurations:\n").String())
		maxTypeLen := 0
		for dataType := range missing {
			if len(dataType) > maxTypeLen {
				maxTypeLen = len(dataType)
			}
		}

		for dataType, values := range missing {
			paddedType := fmt.Sprintf("%-*s", maxTypeLen, dataType)
			errorMsg.WriteString(fmt.Sprintf("  %s: %s\n",
				au.Bold(paddedType),
				strings.Join(values, ", ")))
		}
		errorMsg.WriteString(" \n ")
	}
	if len(validation) > 0 {
		errorMsg.WriteString(au.Red("Validation Errors:\n").String())
		for dataType, err := range validation {
			errorMsg.WriteString(fmt.Sprintf("  %s: %s\n", au.Bold(dataType), err.Error()))
		}
		errorMsg.WriteString(" \n ")
	}
	errorMsg.WriteString(LEARN_MORE)
	return errors.Newf(errorMsg.String())
}
