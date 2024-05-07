FROM golang as builder
LABEL org.opencontainers.image.authors="dev@homai.io"

WORKDIR /build
COPY neo4j /build
COPY go.mod go.sum /build/ 
RUN go install github.com/mitchellh/gox@latest
# RUN gox -output=./bin/neo4j-database-plugin -verbose  ./...
RUN go build -o ./bin/neo4j-database-plugin ./...
RUN sha256sum ./bin/neo4j-database-plugin | cut -d' ' -f1 > neo4j-sha256.txt


FROM hashicorp/vault
LABEL org.opencontainers.image.authors="dev@homai.io"
WORKDIR /plugins
COPY --from=builder /build/bin/neo4j-database-plugin .
COPY --from=builder /build/neo4j-sha256.txt .
COPY scripts/register_plugin.sh .
RUN chmod +x neo4j-database-plugin
RUN chmod +x register_plugin.sh
ENV VAULT_LOCAL_CONFIG='{"plugin_directory": "/plugins"}'
ENV VAULT_LOG_LEVEL=trace
ENV VAULT_ADDR='http://0.0.0.0:8200'