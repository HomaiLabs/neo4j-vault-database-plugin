FROM golang as builder
LABEL org.opencontainers.image.authors="dev@homai.io"

WORKDIR /build
COPY neo4j /build
COPY go.mod go.sum /build/
RUN go install github.com/mitchellh/gox@latest
# RUN gox -output=./bin/neo4j-vault-database-plugin -verbose  ./...
RUN go build -o ./bin/neo4j-vault-database-plugin ./...



FROM hashicorp/vault
LABEL org.opencontainers.image.authors="dev@homai.io"
WORKDIR /plugins
COPY --from=builder /build/bin/neo4j-vault-database-plugin .
ENV VAULT_LOCAL_CONFIG='{"plugin_directory": "/plugins"}' 