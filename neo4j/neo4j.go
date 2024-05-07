package neo4j

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"

	"github.com/hashicorp/go-secure-stdlib/strutil"
	"github.com/hashicorp/vault/sdk/database/dbplugin/v5"
	"github.com/hashicorp/vault/sdk/database/helper/dbutil"
	"github.com/hashicorp/vault/sdk/helper/template"
	"github.com/hashicorp/vault/sdk/logical"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

const (
	neo4jTypeName = "neo4j"

	defaultUserNameTemplate = `{{ printf "v-%s-%s-%s-%s" (.DisplayName | truncate 15) (.RoleName | truncate 15) (random 20) (unix_time) | replace "." "-" | truncate 100 }}`
)

type Neo4j struct {
	*neo4jConnectionProducer

	usernameProducer template.StringTemplate
}

var (
	_ dbplugin.Database       = &Neo4j{}
	_ logical.PluginVersioner = (*Neo4j)(nil)
	// ReportedVersion is used to report a specific version to Vault.
	ReportedVersion = "v1.0.0-beta"
)

// New returns a new neo4j instance

func New() (interface{}, error) {
	log.Println("Running neo4j databse plugin")
	db := new()
	dbType := dbplugin.NewDatabaseErrorSanitizerMiddleware(db, db.secretValues)
	return dbType, nil
}

func new() *Neo4j {
	connProducer := &neo4jConnectionProducer{
		Type: neo4jTypeName,
	}

	return &Neo4j{
		neo4jConnectionProducer: connProducer,
	}
}

// Type returns the TypeName for this backend
func (m *Neo4j) Type() (string, error) {
	return neo4jTypeName, nil
}

func (p *Neo4j) PluginVersion() logical.PluginVersion {
	log.Println("Reading plugin version")
	return logical.PluginVersion{Version: ReportedVersion}
}

func (m *Neo4j) Initialize(ctx context.Context, req dbplugin.InitializeRequest) (dbplugin.InitializeResponse, error) {
	m.Lock()
	defer m.Unlock()

	m.RawConfig = req.Config

	usernameTemplate, err := strutil.GetString(req.Config, "username_template")
	if err != nil {
		return dbplugin.InitializeResponse{}, fmt.Errorf("failed to retrieve username_template: %w", err)
	}
	if usernameTemplate == "" {
		usernameTemplate = defaultUserNameTemplate
	}

	up, err := template.NewTemplate(template.Template(usernameTemplate))
	if err != nil {
		return dbplugin.InitializeResponse{}, fmt.Errorf("unable to initialize username template: %w", err)
	}
	m.usernameProducer = up

	_, err = m.usernameProducer.Generate(dbplugin.UsernameMetadata{})
	if err != nil {
		return dbplugin.InitializeResponse{}, fmt.Errorf("invalid username template: %w", err)
	}

	err = m.neo4jConnectionProducer.loadConfig(req.Config)
	if err != nil {
		return dbplugin.InitializeResponse{}, err
	}

	// Set initialized to true at this point since all fields are set,
	// and the connection can be established at a later time.
	m.Initialized = true

	if req.VerifyConnection {
		client, err := m.neo4jConnectionProducer.createClient(ctx)
		if err != nil {
			return dbplugin.InitializeResponse{}, fmt.Errorf("failed to verify connection: %w", err)
		}

		err = client.VerifyConnectivity(ctx)
		if err != nil {
			_ = client.Close(ctx) // Try to prevent any sort of resource leak
			return dbplugin.InitializeResponse{}, fmt.Errorf("failed to verify connection: %w", err)
		}
		m.neo4jConnectionProducer.client = client
	}

	resp := dbplugin.InitializeResponse{
		Config: req.Config,
	}
	return resp, nil
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

func (m *Neo4j) DeleteUser(ctx context.Context, req dbplugin.DeleteUserRequest) (dbplugin.DeleteUserResponse, error) {
	dropUserCommand := dropUserCommand{
		Username: req.Username,
	}
	var command, params = dropUserCommand.transform()
	if err := m.runCommandWithRetry(ctx, command, params); err != nil {
		return dbplugin.DeleteUserResponse{}, err
	}
	return dbplugin.DeleteUserResponse{}, nil
}

func (m *Neo4j) UpdateUser(ctx context.Context, req dbplugin.UpdateUserRequest) (dbplugin.UpdateUserResponse, error) {
	if req.Password != nil {
		err := m.changeUserPassword(ctx, req.Username, req.Password.NewPassword)
		return dbplugin.UpdateUserResponse{}, err
	}
	return dbplugin.UpdateUserResponse{}, nil
}

func (m *Neo4j) changeUserPassword(ctx context.Context, username, password string) error {

	// Currently doesn't support custom statements for changing the user's password
	changeUserCmd := &updateUserCommand{
		Username: username,
		Password: password,
	}

	var command, params = changeUserCmd.transform()
	return m.runCommandWithRetry(ctx, command, params)
}

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
