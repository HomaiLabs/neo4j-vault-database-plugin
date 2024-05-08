// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package neo4j

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/vault/sdk/helper/docker"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

const (
	Neo4jUsername = "neo4j"
	Neo4jPassword = "a_secure_password"
)
// PrepareTestContainer calls PrepareTestContainerWithDatabase without a
// database name value, which results in configuring a database named "test"
func PrepareTestContainer(t *testing.T, version string) (cleanup func(), retURL string) {
	return PrepareTestContainerWithDatabase(t, version, "default")
}

// PrepareTestContainerWithDatabase configures a test container with a given
// database name, to test non-test/admin database configurations
func PrepareTestContainerWithDatabase(t *testing.T, version, dbName string) (func(), string) {
	if os.Getenv("NEO4J_URL") != "" {
		return func() {}, os.Getenv("NEO4J_URL")
	}
	
	runner, err := docker.NewServiceRunner(docker.RunOptions{
		ContainerName: "neo4j",
		ImageRepo:     "docker.io/library/neo4j",
		ImageTag:      version,
		Ports:         []string{"7687/tcp"},
		Env: 		   []string{fmt.Sprintf("NEO4J_AUTH=%s/%s", Neo4jUsername, Neo4jPassword)},
	})
	if err != nil {
		t.Fatalf("could not start docker neo4j: %s", err)
	}

	svc, err := runner.StartService(context.Background(), func(ctx context.Context, host string, port int) (docker.ServiceConfig, error) {
		connURL := fmt.Sprintf("neo4j://%s:%d", host, port)
		if dbName != "" {
			connURL = fmt.Sprintf("%s/%s", connURL, dbName)
		}

		ctx, _ = context.WithTimeout(context.Background(), 1*time.Minute)
		
		client, err := neo4j.NewDriverWithContext(connURL, neo4j.BasicAuth(Neo4jUsername, Neo4jPassword, ""))

		if err != nil {
			return nil, err
		}

		
		err = client.VerifyConnectivity(ctx)
		
		if err != nil {
			_ = client.Close(ctx) // Try to prevent any sort of resource leak
			return nil, err
		}

		if err = client.Close(ctx); err != nil {
			t.Fatal(err)
		}

		return docker.NewServiceURLParse(connURL)
	})
	if err != nil {
		t.Fatalf("could not start docker neo4j: %s", err)
	}

	return svc.Cleanup, svc.Config.URL().String()
}
