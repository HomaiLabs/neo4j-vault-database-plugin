export VAULT_ADDR='http://0.0.0.0:8200'
SHA256=$(cat /plugins/neo4j-sha256.txt)
vault write sys/plugins/catalog/database/neo4j-vault-database-plugin sha256=$SHA256  command="neo4j-vault-database-plugin" 