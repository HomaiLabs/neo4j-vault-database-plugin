FROM golang:1.21 as builder
LABEL org.opencontainers.image.authors="dev@homai.io"

 

WORKDIR /build
COPY ./ /build/ 
RUN go install github.com/mitchellh/gox@latest
ENV VAULT_DEV_BUILD=x
RUN ./scripts/build.sh
RUN sha256sum ./pkg/linux_arm64/neo4j-vault-database-plugin | cut -d' ' -f1 > neo4j-sha256.txt


FROM hashicorp/vault
LABEL org.opencontainers.image.authors="dev@homai.io"
WORKDIR /plugins
COPY --from=builder /build/pkg/linux_arm64/neo4j-vault-database-plugin .
COPY --from=builder /build/neo4j-sha256.txt .
COPY scripts/register_plugin.sh .
RUN chmod +x neo4j-vault-database-plugin
RUN chmod +x register_plugin.sh
# ENV VAULT_LOCAL_CONFIG='{"plugin_directory": "/plugins"}'
ENV VAULT_LOCAL_CONFIG='{"storage": {"file": {"path": "/vault/file"}}, "listener": [{"tcp": { "address": "0.0.0.0:8200", "tls_disable": true}}], "default_lease_ttl": "168h", "max_lease_ttl": "720h", "ui": true, "plugin_directory": "/plugins"}'
ENV VAULT_LOG_LEVEL=trace
ENV VAULT_ADDR='http://0.0.0.0:8200'
