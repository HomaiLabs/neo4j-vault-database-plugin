package neo4j

import (
	// "log"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/hashicorp/vault/api"
	"github.com/hashicorp/vault/sdk/database/dbplugin/v5"
	"github.com/hashicorp/vault/sdk/database/helper/dbutil"
	"github.com/hashicorp/vault/sdk/helper/template"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
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

func (m *Neo4j) NewUser(ctx context.Context, req dbplugin.NewUserRequest) (dbplugin.NewUserResponse, error) {
	if len(req.Statements.Commands) == 0 {
		return dbplugin.NewUserResponse{}, dbutil.ErrEmptyCreationStatement
	}

	username, err := m.usernameProducer.Generate(req.UsernameConfig)
	if err != nil {
		return dbplugin.NewUserResponse{}, err
	}

	// Unmarshal statements.CreationStatements into neo4jRoles
	var neo4jCS neo4jStatement
	err = json.Unmarshal([]byte(req.Statements.Commands[0]), &neo4jCS)
	if err != nil {
		return dbplugin.NewUserResponse{}, err
	}

	// Default to "admin" if no db provided
	if neo4jCS.DB == "" {
		neo4jCS.DB = "admin"
	}

	if len(neo4jCS.Roles) == 0 {
		return dbplugin.NewUserResponse{}, fmt.Errorf("roles array is required in creation statement")
	}

	createUserCmd := createUserCommand{
		Username: username,
		Password: req.Password,
		Roles:    neo4jCS.Roles.toStandardRolesArray(),
	}
	var command, params = createUserCmd.transform()

	if err := m.runCommandWithRetry(ctx, command, params); err != nil {
		return dbplugin.NewUserResponse{}, err
	}

	resp := dbplugin.NewUserResponse{
		Username: username,
	}
	return resp, nil
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

// runCommandWithRetry runs a command and retries once more if there's a failure
// on the first attempt. This should be called with the lock held
func (m *Neo4j) runCommandWithRetry(ctx context.Context, command string, params map[string]any) error {
	// Get the client
	client, err := m.Connection(ctx)
	if err != nil {
		return err
	}

	defer client.Close(ctx)

	err = executeWrite(client, ctx, command, params)

	// Error check on the first attempt
	switch {
	case err == nil:
		return nil
	case err == io.EOF, strings.Contains(err.Error(), "EOF"):
		// Call getConnection to reset and retry query if we get an EOF error on first attempt.
		client, err = m.Connection(ctx)
		if err != nil {
			return err
		}
		err = executeWrite(client, ctx, command, params)
		if err != nil {
			return err
		}
	default:
		return err
	}

	return nil
}

func executeWrite(client neo4j.SessionWithContext, ctx context.Context, command string, params map[string]any) error {
	_, err := client.ExecuteWrite(ctx, func(transaction neo4j.ManagedTransaction) (any, error) {
		result, err := transaction.Run(ctx,
			command,
			params)
		if err != nil {
			return nil, err
		}

		if result.Next(ctx) {
			return result.Record().Values[0], nil
		}

		return nil, result.Err()
	})
	return err
}
