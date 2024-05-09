DOCKER_IMAGE_NAME:=vault-neo4j
VAULT_PORT=8200

docker-build:
	docker build . -t ${DOCKER_IMAGE_NAME}

docker-run:
	docker run --cap-add=IPC_LOCK -e 'VAULT_DEV_ROOT_TOKEN_ID=myroot' -e 'VAULT_DEV_LISTEN_ADDRESS=127.0.0.1:8300' -p ${VAULT_PORT}:${VAULT_PORT} ${DOCKER_IMAGE_NAME}

docker-run-server:
	docker run --cap-add=IPC_LOCK  -p ${VAULT_PORT}:${VAULT_PORT} ${DOCKER_IMAGE_NAME} server

build:
	./scripts/build.sh	