package rtconfgen

import (
	"testing"

	"google.golang.org/protobuf/types/known/timestamppb"

	"encore.dev/appruntime/exported/config"
	runtimev1 "encr.dev/proto/encore/runtime/v1"
)

func TestToLegacySQLAuthMethod(t *testing.T) {
	tests := []struct {
		name   string
		method runtimev1.SQLAuthMethod
		want   config.SQLAuthMethod
	}{
		{"password", runtimev1.SQLAuthMethod_SQL_AUTH_METHOD_PASSWORD, config.SQLAuthPassword},
		{"gcp_iam", runtimev1.SQLAuthMethod_SQL_AUTH_METHOD_GCP_IAM, config.SQLAuthGCPIAM},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := ToLegacy(sqlRuntimeConfig(test.method), nil)
			if err != nil {
				t.Fatalf("ToLegacy: %v", err)
			}
			if len(got.SQLDatabases) != 1 {
				t.Fatalf("got %d databases, want 1", len(got.SQLDatabases))
			}
			if db := got.SQLDatabases[0]; db.AuthMethod != test.want {
				t.Errorf("got auth method %q, want %q", db.AuthMethod, test.want)
			}
		})
	}

	if _, err := ToLegacy(sqlRuntimeConfig(runtimev1.SQLAuthMethod(42)), nil); err == nil {
		t.Errorf("got no error for an unknown auth method, want one")
	}
}

func sqlRuntimeConfig(method runtimev1.SQLAuthMethod) *runtimev1.RuntimeConfig {
	return &runtimev1.RuntimeConfig{
		Environment: &runtimev1.Environment{
			EnvType: runtimev1.Environment_TYPE_PRODUCTION,
			Cloud:   runtimev1.Environment_CLOUD_GCP,
		},
		Deployment: &runtimev1.Deployment{
			DeployedAt:       timestamppb.Now(),
			ServiceDiscovery: &runtimev1.ServiceDiscovery{},
			Observability:    &runtimev1.Observability{},
		},
		Infra: &runtimev1.Infrastructure{
			Resources: &runtimev1.Infrastructure_Resources{
				SqlClusters: []*runtimev1.SQLCluster{{
					Rid: "cluster",
					Servers: []*runtimev1.SQLServer{{
						Rid:  "server",
						Host: "10.0.0.3:5432",
						Kind: runtimev1.ServerKind_SERVER_KIND_PRIMARY,
					}},
					Databases: []*runtimev1.SQLDatabase{{
						Rid:        "db",
						EncoreName: "db",
						CloudName:  "db",
						ConnPools:  []*runtimev1.SQLConnectionPool{{RoleRid: "role"}},
					}},
				}},
			},
			Credentials: &runtimev1.Infrastructure_Credentials{
				SqlRoles: []*runtimev1.SQLRole{{
					Rid:        "role",
					Username:   "my-sa@my-project.iam",
					AuthMethod: method,
				}},
			},
		},
	}
}
