DOCKER_IMAGE_NAME:=vault-neo4j

docker-built:
	docker build . -t ${DOCKER_IMAGE_NAME}

docker-run:
	docker run --cap-add=IPC_LOCK -e 'VAULT_DEV_ROOT_TOKEN_ID=myroot' -e 'VAULT_DEV_LISTEN_ADDRESS=127.0.0.1:8300' ${DOCKER_IMAGE_NAME}

docker-run-server:
	docker run --cap-add=IPC_LOCK  -p 8200:8200 ${DOCKER_IMAGE_NAME} server

build:
	./scripts/build.sh	