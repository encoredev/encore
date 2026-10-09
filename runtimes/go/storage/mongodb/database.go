package mongodb

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"sync"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"encore.dev/appruntime/exported/config"
)

// Database represents a MongoDB database.
//
// See NewDatabase for more information on how to declare a Database.
type Database struct {
	name string
	mgr  *Manager

	// dbNameOverride, if set, is the name of the database on the server
	// to use instead of the configured one. It is set for test databases.
	dbNameOverride string

	noopDB bool // true if this is a dummy database that does nothing and returns errors for all operations

	initOnce sync.Once
	client   *mongo.Client
	db       *mongo.Database
}

// DatabaseConfig specifies configuration for declaring a new MongoDB database.
type DatabaseConfig struct{}

var errNoopDB = errors.New("mongodb: this service is not configured to use this database. Use mongodb.Named in this service to get a reference and access to the database from this service")

// init connects to the database the first time it is used.
func (db *Database) init() {
	if db.noopDB {
		return
	}

	db.initOnce.Do(func() {
		srv, dbCfg, found := db.mgr.lookup(db.name)
		if !found {
			db.noopDB = true
			return
		}

		opts, err := clientOptions(srv, dbCfg)
		if err != nil {
			panic("mongodb: " + err.Error())
		}

		// Connect does not block: the driver connects in the background
		// and the first operation waits for a usable server.
		client, err := mongo.Connect(opts)
		if err != nil {
			panic("mongodb: setup db: " + err.Error())
		}
		dbName := dbCfg.DatabaseName
		if db.dbNameOverride != "" {
			dbName = db.dbNameOverride
		}
		db.client = client
		db.db = client.Database(dbName)
	})
}

// clientOptions computes the MongoDB client options for a database config.
func clientOptions(srv *config.MongoServer, db *config.MongoDatabase) (*options.ClientOptions, error) {
	opts := options.Client().
		SetHosts(srv.Hosts).
		SetDirect(srv.DirectConnection)

	if srv.ReplicaSet != "" {
		opts.SetReplicaSet(srv.ReplicaSet)
	}

	if db.User != "" {
		opts.SetAuth(options.Credential{
			Username:   db.User,
			Password:   db.Password,
			AuthSource: db.AuthSource,
		})
	}

	// Set the pool size based on the config.
	opts.SetMaxPoolSize(30)
	if n := db.MaxConnections; n > 0 {
		opts.SetMaxPoolSize(uint64(n))
	}
	if n := db.MinConnections; n > 0 {
		opts.SetMinPoolSize(uint64(n))
	}

	if srv.ServerCACert != "" || srv.ClientCert != "" {
		tlsConfig := &tls.Config{}

		// If we have a server CA, set it in the TLS config.
		if srv.ServerCACert != "" {
			caCertPool := x509.NewCertPool()
			if !caCertPool.AppendCertsFromPEM([]byte(srv.ServerCACert)) {
				return nil, fmt.Errorf("invalid server ca cert")
			}
			tlsConfig.RootCAs = caCertPool
		}

		// If we have a client cert, set it in the TLS config.
		if srv.ClientCert != "" {
			cert, err := tls.X509KeyPair([]byte(srv.ClientCert), []byte(srv.ClientKey))
			if err != nil {
				return nil, fmt.Errorf("parse client cert: %v", err)
			}
			tlsConfig.Certificates = []tls.Certificate{cert}
		}

		opts.SetTLSConfig(tlsConfig)
	}

	if err := opts.Validate(); err != nil {
		return nil, fmt.Errorf("invalid client options: %v", err)
	}
	return opts, nil
}

// Collection returns a handle for the collection with the given name.
// MongoDB creates the collection when the first document is inserted.
func (db *Database) Collection(name string) *Collection {
	db.init()
	if db.noopDB {
		return &Collection{err: errNoopDB}
	}
	return &Collection{db: db, coll: db.db.Collection(name)}
}

// WithTransaction runs fn in a transaction, and commits it if fn returns nil.
// All operations inside fn must use the ctx passed to fn.
//
// The transaction is retried if MongoDB reports a transient error,
// so fn may be called more than once.
func (db *Database) WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	db.init()
	if db.noopDB {
		return errNoopDB
	}

	sess, err := db.client.StartSession()
	if err != nil {
		return err
	}
	defer sess.EndSession(ctx)

	end := db.traceStart("", "withTransaction", nil)
	_, err = sess.WithTransaction(ctx, func(ctx context.Context) (any, error) {
		return nil, fn(ctx)
	})
	end(err)
	return convertErr(err)
}

// Driver returns the underlying MongoDB driver database,
// for operations this package does not provide.
// It returns nil if this service is not configured to use the database.
func (db *Database) Driver() *mongo.Database {
	db.init()
	if db.noopDB {
		return nil
	}
	return db.db
}

func (db *Database) shutdown(ctx context.Context) {
	if db.client != nil {
		_ = db.client.Disconnect(ctx)
	}
}
