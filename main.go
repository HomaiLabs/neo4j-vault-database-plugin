package main

import (
	"github.com/HomaiLabs/neo4j-vault-database-plugin/neo4j"
	"log"
	"os"

	dbplugin "github.com/hashicorp/vault/sdk/database/dbplugin/v5"
)

func main() {
	err := Run()
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}
}

// Run instantiates a Neo4j object, and runs the RPC server for the plugin
func Run() error {
	dbplugin.ServeMultiplex(neo4j.New)

	return nil
}
