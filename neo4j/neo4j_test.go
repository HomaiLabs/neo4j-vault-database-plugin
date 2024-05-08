// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package neo4j

import (
	"context"
	// "crypto/tls"
	// "crypto/x509"
	// "fmt"
	// "net/http"
	"reflect"
	// "strings"
	// "sync"
	"testing"
	"time"

	// "github.com/google/go-cmp/cmp"
	// "github.com/google/go-cmp/cmp/cmpopts"
	// "github.com/hashicorp/vault/helper/testhelpers/certhelpers"
	"github.com/HomaiLabs/neo4j-vault-database-plugin/helper/testhelpers"
	dbplugin "github.com/hashicorp/vault/sdk/database/dbplugin/v5"
	dbtesting "github.com/hashicorp/vault/sdk/database/dbplugin/v5/testing"
	"github.com/stretchr/testify/require"
	// "go.mongodb.org/mongo-driver/mongo"
	// "go.mongodb.org/mongo-driver/mongo/options"
	// "go.mongodb.org/mongo-driver/mongo/readpref"
	neo4jDB "github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

const (
	neo4jAdminRole       = `{ "db": "admin", "roles": [ { "role": "readWrite" } ] }`
	neo4jTestDBAdminRole = `{ "db": "test", "roles": [ { "role": "readWrite" } ] }`
)

func TestNeo4j_Initialize(t *testing.T) {
	cleanup, connURL := neo4j.PrepareTestContainer(t, "latest")
	defer cleanup()

	db := new()
	defer dbtesting.AssertClose(t, db)

	config := map[string]interface{}{
		"connection_url": connURL,
		"username": neo4j.Neo4jUsername,
		"password": neo4j.Neo4jPassword,
	}
	

	// // Make a copy since the original map could be modified by the Initialize call
	expectedConfig := copyConfig(config)

	req := dbplugin.InitializeRequest{
		Config:           config,
		VerifyConnection: true,
	}

	resp := dbtesting.AssertInitialize(t, db, req)

	if !reflect.DeepEqual(resp.Config, expectedConfig) {
		t.Fatalf("Actual config: %#v\nExpected config: %#v", resp.Config, expectedConfig)
	}

	if !db.Initialized {
		t.Fatal("Database should be initialized")
	}
}


func TestNewUser_usernameTemplate(t *testing.T) {
	type testCase struct {
		usernameTemplate string

		newUserReq            dbplugin.NewUserRequest
		expectedUsernameRegex string
	}

	tests := map[string]testCase{
		"default username template": {
			usernameTemplate: "",

			newUserReq: dbplugin.NewUserRequest{
				UsernameConfig: dbplugin.UsernameMetadata{
					DisplayName: "token",
					RoleName:    "testrolenamewithmanycharacters",
				},
				Statements: dbplugin.Statements{
					Commands: []string{neo4jAdminRole},
				},
				Password:   "98yq3thgnakjsfhjkl",
				Expiration: time.Now().Add(time.Minute),
			},

			expectedUsernameRegex: "^v-token-testrolenamewit-[a-zA-Z0-9]{20}-[0-9]{10}$",
		},
		"default username template with invalid chars": {
			usernameTemplate: "",

			newUserReq: dbplugin.NewUserRequest{
				UsernameConfig: dbplugin.UsernameMetadata{
					DisplayName: "a.bad.account",
					RoleName:    "a.bad.role",
				},
				Statements: dbplugin.Statements{
					Commands: []string{neo4jAdminRole},
				},
				Password:   "98yq3thgnakjsfhjkl",
				Expiration: time.Now().Add(time.Minute),
			},

			expectedUsernameRegex: "^v-a-bad-account-a-bad-role-[a-zA-Z0-9]{20}-[0-9]{10}$",
		},
		"custom username template": {
			usernameTemplate: "{{random 2 | uppercase}}_{{unix_time}}_{{.RoleName | uppercase}}_{{.DisplayName | uppercase}}",

			newUserReq: dbplugin.NewUserRequest{
				UsernameConfig: dbplugin.UsernameMetadata{
					DisplayName: "token",
					RoleName:    "testrolenamewithmanycharacters",
				},
				Statements: dbplugin.Statements{
					Commands: []string{neo4jAdminRole},
				},
				Password:   "98yq3thgnakjsfhjkl",
				Expiration: time.Now().Add(time.Minute),
			},

			expectedUsernameRegex: "^[A-Z0-9]{2}_[0-9]{10}_TESTROLENAMEWITHMANYCHARACTERS_TOKEN$",
		},
		"admin in test database username template": {
			usernameTemplate: "",

			newUserReq: dbplugin.NewUserRequest{
				UsernameConfig: dbplugin.UsernameMetadata{
					DisplayName: "token",
					RoleName:    "testrolenamewithmanycharacters",
				},
				Statements: dbplugin.Statements{
					Commands: []string{neo4jTestDBAdminRole},
				},
				Password:   "98yq3thgnakjsfhjkl",
				Expiration: time.Now().Add(time.Minute),
			},

			expectedUsernameRegex: "^v-token-testrolenamewit-[a-zA-Z0-9]{20}-[0-9]{10}$",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			cleanup, connURL := neo4j.PrepareTestContainer(t, "latest")
			defer cleanup()


			db := new()
			defer dbtesting.AssertClose(t, db)

			initReq := dbplugin.InitializeRequest{
				Config: map[string]interface{}{
					"connection_url":    connURL,
					"username": neo4j.Neo4jUsername,
					"password": neo4j.Neo4jPassword,
					"username_template": test.usernameTemplate,
				},
				VerifyConnection: true,
			}
			dbtesting.AssertInitialize(t, db, initReq)

			ctx := context.Background()
			newUserResp, err := db.NewUser(ctx, test.newUserReq)
			require.NoError(t, err)
			require.Regexp(t, test.expectedUsernameRegex, newUserResp.Username)

			err = assertCredsExist(t, newUserResp.Username, test.newUserReq.Password, connURL)
			if err != nil {
				t.Fatalf(err.Error())
			}

		})
	}
}

func TestNeo4j_CreateUser(t *testing.T) {
	cleanup, connURL := neo4j.PrepareTestContainer(t, "latest")
	defer cleanup()

	db := new()
	defer dbtesting.AssertClose(t, db)

	initReq := dbplugin.InitializeRequest{
		Config: map[string]interface{}{
			"connection_url": connURL,
			"username": neo4j.Neo4jUsername,
			"password": neo4j.Neo4jPassword,
		},
		VerifyConnection: true,
	}
	dbtesting.AssertInitialize(t, db, initReq)
	password := "myreallysecurepassword"
	username := "atestuser"
	createResp := createDBUser(t, db, username, password)
	err := assertCredsExist(t, createResp.Username, password, connURL)
	if err != nil {
		t.Fatalf(err.Error())
	}
}


func TestNeo4j_DeleteUser(t *testing.T) {
	cleanup, connURL := neo4j.PrepareTestContainer(t, "latest")
	defer cleanup()

	db := new()
	defer dbtesting.AssertClose(t, db)

	initReq := dbplugin.InitializeRequest{
		Config: map[string]interface{}{
			"connection_url": connURL,
			"username": neo4j.Neo4jUsername,
			"password": neo4j.Neo4jPassword,
		},
		VerifyConnection: true,
	}
	dbtesting.AssertInitialize(t, db, initReq)
	password := "myreallysecurepassword"
	username := "atestuser"
	createResp := createDBUser(t, db, username, password)
	err := assertCredsExist(t, createResp.Username, password, connURL)
	if err != nil {
		t.Fatalf(err.Error())
	}
	// Test default revocation statement
	delReq := dbplugin.DeleteUserRequest{
		Username: createResp.Username,
	}

	dbtesting.AssertDeleteUser(t, db, delReq)

	err = assertCredsDoNotExist(t, createResp.Username, password, connURL)
	if err != nil {
		t.Fatalf(err.Error())
	}
}

func TestNeo4j_UpdateUser_Password(t *testing.T) {
	cleanup, connURL := neo4j.PrepareTestContainer(t, "latest")
	defer cleanup()

	
	db := new()
	defer dbtesting.AssertClose(t, db)

	initReq := dbplugin.InitializeRequest{
		Config: map[string]interface{}{
			"connection_url": connURL,
			"username": neo4j.Neo4jUsername,
			"password": neo4j.Neo4jPassword,
		},
		VerifyConnection: true,
	}
	dbtesting.AssertInitialize(t, db, initReq)

	// create the database user in advance, and test the connection
	dbUser := "testneo4jouser"
	startingPassword := "myfirstpassword"
	createResp := createDBUser(t, db, dbUser, startingPassword)
	err := assertCredsExist(t, createResp.Username, startingPassword, connURL)
	if err != nil {
		t.Fatalf(err.Error())
	}
	newPassword := "myreallysecurecredentials"

	updateReq := dbplugin.UpdateUserRequest{
		Username: createResp.Username,
		Password: &dbplugin.ChangePassword{
			NewPassword: newPassword,
		},
	}
	dbtesting.AssertUpdateUser(t, db, updateReq)

	err = assertCredsExist(t, createResp.Username, newPassword, connURL)
	if err != nil {
		t.Fatalf(err.Error())
	}
}

// func TestMongoDB_RotateRoot_NonAdminDB(t *testing.T) {
// 	cleanup, connURL := mongodb.PrepareTestContainer(t, "latest")
// 	defer cleanup()

// 	connURL = connURL + "/test?authSource=test"
// 	db := new()
// 	defer dbtesting.AssertClose(t, db)

// 	initReq := dbplugin.InitializeRequest{
// 		Config: map[string]interface{}{
// 			"connection_url": connURL,
// 		},
// 		VerifyConnection: true,
// 	}
// 	dbtesting.AssertInitialize(t, db, initReq)

// 	dbUser := "testmongouser"
// 	startingPassword := "password"
// 	createDBUser(t, connURL, "test", dbUser, startingPassword)

// 	newPassword := "myreallysecurecredentials"

// 	updateReq := dbplugin.UpdateUserRequest{
// 		Username: dbUser,
// 		Password: &dbplugin.ChangePassword{
// 			NewPassword: newPassword,
// 		},
// 	}
// 	dbtesting.AssertUpdateUser(t, db, updateReq)

// 	assertCredsExist(t, dbUser, newPassword, connURL)
// }

// func TestGetTLSAuth(t *testing.T) {
// 	ca := certhelpers.NewCert(t,
// 		certhelpers.CommonName("certificate authority"),
// 		certhelpers.IsCA(true),
// 		certhelpers.SelfSign(),
// 	)
// 	cert := certhelpers.NewCert(t,
// 		certhelpers.CommonName("test cert"),
// 		certhelpers.Parent(ca),
// 	)

// 	type testCase struct {
// 		username   string
// 		tlsCAData  []byte
// 		tlsKeyData []byte

// 		expectOpts *options.ClientOptions
// 		expectErr  bool
// 	}

// 	tests := map[string]testCase{
// 		"no TLS data set": {
// 			expectOpts: nil,
// 			expectErr:  false,
// 		},
// 		"bad CA": {
// 			tlsCAData: []byte("foobar"),

// 			expectOpts: nil,
// 			expectErr:  true,
// 		},
// 		"bad key": {
// 			tlsKeyData: []byte("foobar"),

// 			expectOpts: nil,
// 			expectErr:  true,
// 		},
// 		"good ca": {
// 			tlsCAData: cert.Pem,

// 			expectOpts: options.Client().
// 				SetTLSConfig(
// 					&tls.Config{
// 						RootCAs: appendToCertPool(t, x509.NewCertPool(), cert.Pem),
// 					},
// 				),
// 			expectErr: false,
// 		},
// 		"good key": {
// 			username:   "unittest",
// 			tlsKeyData: cert.CombinedPEM(),

// 			expectOpts: options.Client().
// 				SetTLSConfig(
// 					&tls.Config{
// 						Certificates: []tls.Certificate{cert.TLSCert},
// 					},
// 				).
// 				SetAuth(options.Credential{
// 					AuthMechanism: "MONGODB-X509",
// 					Username:      "unittest",
// 				}),
// 			expectErr: false,
// 		},
// 	}

// 	for name, test := range tests {
// 		t.Run(name, func(t *testing.T) {
// 			c := new()
// 			c.Username = test.username
// 			c.TLSCAData = test.tlsCAData
// 			c.TLSCertificateKeyData = test.tlsKeyData

// 			actual, err := c.getTLSAuth()
// 			if test.expectErr && err == nil {
// 				t.Fatalf("err expected, got nil")
// 			}
// 			if !test.expectErr && err != nil {
// 				t.Fatalf("no error expected, got: %s", err)
// 			}
// 			assertDeepEqual(t, test.expectOpts, actual)
// 		})
// 	}
// }

// func appendToCertPool(t *testing.T, pool *x509.CertPool, caPem []byte) *x509.CertPool {
// 	t.Helper()

// 	ok := pool.AppendCertsFromPEM(caPem)
// 	if !ok {
// 		t.Fatalf("Unable to append cert to cert pool")
// 	}
// 	return pool
// }

// var cmpClientOptionsOpts = cmp.Options{
// 	cmpopts.IgnoreTypes(http.Transport{}),

// 	cmp.AllowUnexported(options.ClientOptions{}),

// 	cmp.AllowUnexported(tls.Config{}),
// 	cmpopts.IgnoreTypes(sync.Mutex{}, sync.RWMutex{}),

// 	// 'lazyCerts' has a func field which can't be compared.
// 	cmpopts.IgnoreFields(x509.CertPool{}, "lazyCerts"),
// 	cmp.AllowUnexported(x509.CertPool{}),
// }

// // Need a special comparison for ClientOptions because reflect.DeepEquals won't work in Go 1.16.
// // See: https://github.com/golang/go/issues/45891
// func assertDeepEqual(t *testing.T, a, b *options.ClientOptions) {
// 	t.Helper()

// 	if diff := cmp.Diff(a, b, cmpClientOptionsOpts); diff != "" {
// 		t.Fatalf("assertion failed: values are not equal\n--- expected\n+++ actual\n%v", diff)
// 	}
// }

func createDBUser(t *testing.T, db *Neo4j, username string, password string) dbplugin.NewUserResponse {
	

	
	createReq := dbplugin.NewUserRequest{
		UsernameConfig: dbplugin.UsernameMetadata{
			DisplayName: username,
			RoleName:    username,
		},
		Statements: dbplugin.Statements{
			Commands: []string{neo4jAdminRole},
		},
		Password:   password,
		Expiration: time.Now().Add(time.Minute),
	}
	createResp := dbtesting.AssertNewUser(t, db, createReq)
	return createResp
}

func assertCredsExist(t testing.TB, username, password, connURL string) error{
	t.Helper()

	
	var ctx, _ = context.WithTimeout(context.Background(), 1*time.Minute)
	
	client, err := neo4jDB.NewDriverWithContext(connURL, neo4jDB.BasicAuth(username, password, ""))

	if err != nil {
		t.Error(err.Error())
	}

	
	err = client.VerifyConnectivity(ctx)
	
	if err != nil {
		_ = client.Close(ctx) // Try to prevent any sort of resource leak
		return err
	}

	if err = client.Close(ctx); err != nil {
		return err
	}
	return nil
}

func assertCredsDoNotExist(t testing.TB, username, password, connURL string) error {
	t.Helper()

	
	var ctx, _ = context.WithTimeout(context.Background(), 1*time.Minute)
	
	client, err := neo4jDB.NewDriverWithContext(connURL, neo4jDB.BasicAuth(username, password, ""))

	if err != nil {
		t.Error(err.Error())
	}

	
	err = client.VerifyConnectivity(ctx)
	
	if err != nil {
		_ = client.Close(ctx) // Try to prevent any sort of resource leak
		return nil
	}

	if err = client.Close(ctx); err != nil {
		return err
	}

	t.Fatalf("User %q exists and was able to authenticate", username)
	return nil
}

func copyConfig(config map[string]interface{}) map[string]interface{} {
	newConfig := map[string]interface{}{}
	for k, v := range config {
		newConfig[k] = v
	}
	return newConfig
}
