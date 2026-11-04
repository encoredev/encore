package infra

import (
	"strings"
	"testing"
)

func TestSQLDatabaseAuthMethodValidation(t *testing.T) {
	tests := []struct {
		name    string
		tls     *TLSConfig
		db      SQLDatabase
		wantErr string // a substring of the error at wantPath, or "" for none
		path    string
	}{
		{
			name: "password",
			db:   SQLDatabase{Username: EnvString{Str: "owner"}, Password: EnvString{Str: "secret"}},
		},
		{
			name: "explicit password",
			db:   SQLDatabase{Username: EnvString{Str: "owner"}, Password: EnvString{Str: "secret"}, AuthMethod: "password"},
		},
		{
			name:    "password missing",
			db:      SQLDatabase{Username: EnvString{Str: "owner"}},
			path:    "password",
			wantErr: "Must not be empty",
		},
		{
			name: "gcp_iam",
			db:   SQLDatabase{Username: EnvString{Str: "my-sa@my-project.iam"}, AuthMethod: "gcp_iam"},
		},
		{
			name:    "gcp_iam with a password",
			db:      SQLDatabase{Username: EnvString{Str: "my-sa@my-project.iam"}, Password: EnvString{Str: "secret"}, AuthMethod: "gcp_iam"},
			path:    "password",
			wantErr: "Must be empty with auth_method gcp_iam",
		},
		{
			name:    "gcp_iam without TLS",
			tls:     &TLSConfig{Disabled: true},
			db:      SQLDatabase{Username: EnvString{Str: "my-sa@my-project.iam"}, AuthMethod: "gcp_iam"},
			path:    "auth_method",
			wantErr: "needs TLS",
		},
		{
			name:    "unknown",
			db:      SQLDatabase{Username: EnvString{Str: "owner"}, Password: EnvString{Str: "secret"}, AuthMethod: "kerberos"},
			path:    "auth_method",
			wantErr: "Must be one of",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db := test.db
			_, errs := Validate(&SQLServer{
				Host:      "10.0.0.3:5432",
				TLSConfig: test.tls,
				Databases: map[string]*SQLDatabase{"mydb": &db},
			})
			if test.wantErr == "" {
				if len(errs) > 0 {
					t.Fatalf("got errors %v, want none", errs)
				}
				return
			}
			for path, err := range errs {
				if strings.HasSuffix(path.String(), "."+test.path) && strings.Contains(err.Error(), test.wantErr) {
					return
				}
			}
			t.Fatalf("got errors %v, want %q at %s", errs, test.wantErr, test.path)
		})
	}
}
