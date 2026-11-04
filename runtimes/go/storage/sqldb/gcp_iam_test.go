package sqldb

import (
	"context"
	"testing"

	"golang.org/x/oauth2"

	"encore.dev/appruntime/exported/config"
)

func TestDBConfGCPIAM(t *testing.T) {
	srv := &config.SQLServer{Host: "10.0.0.3:5432"}
	db := &config.SQLDatabase{
		EncoreName:   "ignore",
		DatabaseName: "dbname",
		User:         "my-sa@my-project.iam",
		AuthMethod:   config.SQLAuthGCPIAM,
	}

	cfg, err := dbConf(srv, db, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ConnConfig.User != "my-sa@my-project.iam" {
		t.Errorf("got user %q, want the IAM user", cfg.ConnConfig.User)
	}
	// With no password, the database name must not be read as one.
	if cfg.ConnConfig.Database != "dbname" || cfg.ConnConfig.Password != "" {
		t.Errorf("got database %q and password %q, want %q and none", cfg.ConnConfig.Database, cfg.ConnConfig.Password, "dbname")
	}
	// Without a server CA the connection would otherwise prefer TLS but fall
	// back to plaintext, which would send the token in the clear.
	if cfg.ConnConfig.TLSConfig == nil {
		t.Errorf("got no TLS config, want TLS required")
	}
	for _, fb := range cfg.ConnConfig.Fallbacks {
		if fb.TLSConfig == nil {
			t.Errorf("got a plaintext fallback to %s:%d, want none", fb.Host, fb.Port)
		}
	}
	if cfg.BeforeConnect == nil {
		t.Fatalf("got no BeforeConnect, want one setting the token")
	}

	orig := gcpIAMTokens
	t.Cleanup(func() { gcpIAMTokens = orig })
	gcpIAMTokens = func() (oauth2.TokenSource, error) {
		return oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "ya29.token"}), nil
	}

	connCfg := cfg.ConnConfig.Copy()
	if err := cfg.BeforeConnect(context.Background(), connCfg); err != nil {
		t.Fatalf("BeforeConnect: %v", err)
	}
	if connCfg.Password != "ya29.token" {
		t.Errorf("got password %q, want the access token", connCfg.Password)
	}
}

func TestDBConfPasswordHasNoBeforeConnect(t *testing.T) {
	cfg, err := dbConf(&config.SQLServer{Host: "hostname"}, &config.SQLDatabase{
		DatabaseName: "dbname",
		User:         "user",
		Password:     "password",
	}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.BeforeConnect != nil {
		t.Errorf("got a BeforeConnect, want none")
	}
}

func TestDBConfUnknownAuthMethod(t *testing.T) {
	_, err := dbConf(&config.SQLServer{Host: "hostname"}, &config.SQLDatabase{
		DatabaseName: "dbname",
		User:         "user",
		AuthMethod:   "kerberos",
	}, "")
	if err == nil {
		t.Fatalf("got no error, want one for the unknown auth method")
	}
}
