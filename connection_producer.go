package neo4j

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/hashicorp/vault/sdk/database/helper/connutil"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

type neo4jDBConnectionProducer struct {
	ConnectionURL string `json:"connection_url" structs:"connection_url" mapstructure:"connection_url"`
	WriteConcern  string `json:"write_concern" structs:"write_concern" mapstructure:"write_concern"`

	Username string `json:"username" structs:"username" mapstructure:"username"`
	Password string `json:"password" structs:"password" mapstructure:"password"`

	TLSCertificateKeyData []byte `json:"tls_certificate_key" structs:"-" mapstructure:"tls_certificate_key"`
	TLSCAData             []byte `json:"tls_ca"              structs:"-" mapstructure:"tls_ca"`

	SocketTimeout          time.Duration `json:"socket_timeout"           structs:"-" mapstructure:"socket_timeout"`
	ConnectTimeout         time.Duration `json:"connect_timeout"          structs:"-" mapstructure:"connect_timeout"`
	ServerSelectionTimeout time.Duration `json:"server_selection_timeout" structs:"-" mapstructure:"server_selection_timeout"`

	Initialized   bool
	RawConfig     map[string]interface{}
	Type          string
	clientOptions neo4j.SessionConfig
	client        neo4j.DriverWithContext
	sync.Mutex
}

func (c *neo4jDBConnectionProducer) secretValues() map[string]string {
	return map[string]string{
		c.Password: "[password]",
	}
}

// Connection creates or returns an existing a database connection. If the session fails
// on a ping check, the session will be closed and then re-created.
// This method does locks the mutex on its own.
func (c *neo4jDBConnectionProducer) Connection(ctx context.Context) (neo4j.SessionWithContext, error) {
	if !c.Initialized {
		return nil, connutil.ErrNotInitialized
	}

	c.Mutex.Lock()
	defer c.Mutex.Unlock()

	if c.client != nil {
		if err := c.client.VerifyConnectivity(ctx); err == nil {
			return c.client.NewSession(ctx, c.clientOptions), nil
		}
		// Ignore error on purpose since we want to re-create a session
		_ = c.client.Close(ctx)
	}

	client, err := c.createClient(ctx)
	if err != nil {
		return nil, err
	}
	c.client = client
	return c.client.NewSession(ctx, c.clientOptions), nil
}

func (c *neo4jDBConnectionProducer) createClient(ctx context.Context) (neo4j.DriverWithContext, error) {
	if !c.Initialized {
		return nil, fmt.Errorf("failed to create client: connection producer is not initialized")
	}

	client, err := neo4j.NewDriverWithContext(c.ConnectionURL, neo4j.BasicAuth(c.Username, c.Password, ""))

	if err != nil {
		return nil, err
	}
	return client, nil
}

// Close terminates the database connection.
func (c *neo4jDBConnectionProducer) Close() error {
	c.Lock()
	defer c.Unlock()

	if c.client != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
		defer cancel()
		if err := c.client.Close(ctx); err != nil {
			return err
		}
	}

	c.client = nil

	return nil
}
