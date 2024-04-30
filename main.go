package neo4j

import (
	// "log"
	"os"

	"github.com/hashicorp/vault/api"

	"github.com/hashicorp/vault/sdk/database/dbplugin/v5"
	"github.com/hashicorp/vault/sdk/helper/template"
	// "context"
	// "github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

func main() {
	apiClientMeta := &api.PluginAPIClientMeta{}
	flags := apiClientMeta.FlagSet()
	flags.Parse(os.Args[1:])

	// err := Run()
	// if err != nil {
	// 	log.Println(err)
	// 	os.Exit(1)
	// }
}

const (
	neo4jDBTypeName = "neo4j"

	defaultUserNameTemplate = `{{ printf "v-%s-%s-%s-%s" (.DisplayName | truncate 15) (.RoleName | truncate 15) (random 20) (unix_time) | replace "." "-" | truncate 100 }}`
)

type Neo4j struct {
	*neo4jDBConnectionProducer

	usernameProducer template.StringTemplate
}

var _ dbplugin.Database = &Neo4j{}

// New returns a new MongoDB instance

func New() (interface{}, error) {
	db := new()
	dbType := dbplugin.NewDatabaseErrorSanitizerMiddleware(db, db.secretValues)
	return dbType, nil
}

func new() *Neo4j {
	connProducer := &neo4jDBConnectionProducer{
		Type: neo4jDBTypeName, // TODO not sure if this is needed
	}

	return &Neo4j{
		neo4jDBConnectionProducer: connProducer,
	}
}

// func Run() error {
// 	ctx := context.Background()
// 	// URI examples: "neo4j://localhost", "neo4j+s://xxx.databases.neo4j.io"
// 	dbUri := "<URI for Neo4j database>"
// 	dbUser := "<Username>"
// 	dbPassword := "<Password>"
// 	driver, err := neo4j.NewDriverWithContext(
// 		dbUri,
// 		neo4j.BasicAuth(dbUser, dbPassword, ""))
// 	defer driver.Close(ctx)

// 	err = driver.VerifyConnectivity(ctx)
// 	if err != nil {
// 		panic(err)
// 	}

// 	return nil
// }

// func New() (interface{}, error) {
// 	// db, err := newDatabase()
// 	// if err != nil {
// 	// 	return nil, err
// 	// }

// 	// This middleware isn't strictly required, but highly recommended to prevent accidentally exposing
// 	// values such as passwords in error messages. An example of this is included below
// 	// db = dbplugin.NewDatabaseErrorSanitizerMiddleware(db, db.secretValues)
// 	return nil, nil
// }

// type MyDatabase struct {
// 	// Variables for the database
// 	password string
// }

// func newDatabase() (MyDatabase, error) {
// 	// ...
// 	db := &MyDatabase{
// 		// ...
// 	}
// 	return db, nil
// }

// func (db *MyDatabase) secretValues() map[string]string {
// 	return map[string]string{
// 		db.password: "[password]",
// 	}
// }
