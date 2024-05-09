# Neo4j HashiCorp Vault Plugin
This vault database plugin implements the [V5 version of Vault database plugin](https://developer.hashicorp.com/vault/docs/secrets/databases/custom)  to support Neo4j.

This project also offers a Docker image which has the Neo4j plugin preconfigured so that it is ready to use.
This code is heavily borrowed from the [MongoDB implementation of the plugin] (https://github.com/hashicorp/vault/tree/main/plugins/database/mongodb).

It's also worth mentioning there already exists another implementation of [this plugin for Neo4j](https://github.com/vivacitylabs/vault-plugin-database-neo4j) but it's based on the older version of the plugin and I was not able to get it to work with the new vault server.  

## Build
to build this project locally you can run (you need to have [Go](https://go.dev/doc/install and [gox](https://github.com/mitchellh/gox) installed)

```sh
make build
```
this will generate a set of cross platform builds which you can choose based on your platform:
- netbsd/386
- windows/386
- darwin/amd64
- freebsd/amd64
- linux/arm
- netbsd/arm
- linux/arm64
- windows/amd64
- solaris/amd64
- linux/amd64
- freebsd/arm
- netbsd/amd64
- openbsd/amd64
- freebsd/386
- openbsd/386
- linux/386

or you can build the docker image via
```
make docker-build
```

## Running [vault server]
If you have the vault server installed you can copy the plugin into plugin directory or run the vault server and point the plugin directory accordingly

```sh
cp pkg/<your_platform>/neo4j-vault-database-plugin /vault-plugins/
vault server -dev -dev-plugin-dir=/vault-plugins -log-level=trace
```
if all goes well you should see an output similar to this

```
neo4j-vault-database-plugin: configuring server automatic mTLS: metadata=true timestamp=2024-05-08T21:39:36.675-0700
plugin process exited: metadata=true path=/vault-plugins/neo4j-vault-database-plugin pid=60345
==> Vault server started! Log data will stream in below:
```

## Running [docker]
run the docker instance
```sh
make docker-run
```
after that you need to register the plugin by hopping on the docker instance
```sh
docker exec -it <docker instance id> sh
vault login
./register_plugin.sh
```

## Neo4j Vault Secret Engine
In order to manage a Neo4j database credentials via vault first you would need to set the Neo4j root credentials in vault.
Let's assume you are running Neo4j on a docker instance

```sh
docker run -dt -p 7474:7474 -p 7687:7687  --env=NEO4J_AUTH=neo4j/my_secret_password neo4j
```

first login to vault
```sh
vault login
```

then you need to first enable the database secret on vault

```
vault secrets enable database
```

and then configure the connection and root username & password
```
vault write database/config/my-neo4j-database \
    plugin_name="neo4j-vault-database-plugin" \
    allowed_roles="my-role" \
    connection_url="neo4j://127.0.0.1:7687" \
    username="neo4j" \
    password="my_secret_password" \
    root_rotation_statements="ALTER USER neo4j SET PASSWORD '${password}' CHANGE NOT REQUIRED"
```    

Then you can create credentials by running the following command
```
vault write database/roles/my-role \
    db_name=my-neo4j-database \
    creation_statements='{ "db": "admin", "roles": [{ "role": "readWrite" }, {"role": "read", "db": "foo"}] }' \
    default_ttl="1h" \
    max_ttl="24h"  
```

check if everything worked as expected
```sh
vault read database/creds/my-role
```
you should see something like
```
Key                Value
---                -----
lease_id           database/creds/my-role/7zFvHP1U4SKIYX8OcFNT3p25
lease_duration     1h
lease_renewable    true
password           CPsKvFnHQ9sR8pL-wwsA
username           v-root-my-role-2GpVgz6BG6LUQhe80sg3-1715236001
```

## Rotating the root password

<p>You can actually rotate the Neo4j root password via the following command.<br>
:warning: Please note after this you will not be able to read this password and only vault knows the root database password. As a result it's suggested to use a separated password for vault than the root password.</p>

```
vault write -force database/rotate-root/my-neo4j-database    
```


## Delete role
```sh
vault delete database/config/my-neo4j-database
``` 


#### Warning


 :warning:  