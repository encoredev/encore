package mongodb

import (
	"slices"
	"testing"

	"encore.dev/appruntime/exported/config"
)

func TestClientOptions(t *testing.T) {
	tests := []struct {
		Srv         *config.MongoServer
		DB          *config.MongoDatabase
		Direct      bool
		ReplicaSet  string
		User        string
		MaxPoolSize uint64
	}{
		{
			// The local MongoDB server started by the Encore daemon.
			Srv: &config.MongoServer{
				Hosts:            []string{"127.0.0.1:55086"},
				ReplicaSet:       "rs0",
				DirectConnection: true,
			},
			DB: &config.MongoDatabase{
				EncoreName:   "ignore",
				DatabaseName: "urls",
			},
			Direct:      true,
			ReplicaSet:  "rs0",
			MaxPoolSize: 30,
		},
		{
			// A self-hosted replica set with credentials.
			Srv: &config.MongoServer{
				Hosts:      []string{"mongo-1:27017", "mongo-2:27017"},
				ReplicaSet: "rs0",
			},
			DB: &config.MongoDatabase{
				EncoreName:     "ignore",
				DatabaseName:   "urls",
				User:           "user",
				Password:       "password",
				AuthSource:     "admin",
				MaxConnections: 20,
			},
			ReplicaSet:  "rs0",
			User:        "user",
			MaxPoolSize: 20,
		},
	}

	for i, test := range tests {
		opts, err := clientOptions(test.Srv, test.DB)
		if err != nil {
			t.Fatalf("test %d: unexpected error: %v", i, err)
		}

		var user string
		if opts.Auth != nil {
			user = opts.Auth.Username
		}
		if !slices.Equal(opts.Hosts, test.Srv.Hosts) {
			t.Fatalf("test %d: got hosts %v, want %v", i, opts.Hosts, test.Srv.Hosts)
		} else if *opts.Direct != test.Direct {
			t.Fatalf("test %d: got direct %v, want %v", i, *opts.Direct, test.Direct)
		} else if got := deref(opts.ReplicaSet); got != test.ReplicaSet {
			t.Fatalf("test %d: got replica set %q, want %q", i, got, test.ReplicaSet)
		} else if user != test.User {
			t.Fatalf("test %d: got user %q, want %q", i, user, test.User)
		} else if *opts.MaxPoolSize != test.MaxPoolSize {
			t.Fatalf("test %d: got max pool size %d, want %d", i, *opts.MaxPoolSize, test.MaxPoolSize)
		}
	}
}

func deref[T any](p *T) T {
	var zero T
	if p == nil {
		return zero
	}
	return *p
}
