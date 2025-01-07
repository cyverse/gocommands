## build docker image
```
docker build --build-arg="GOCOMMANDS_VER=v0.10.9" --tag=cyverse/gocmd:v0.10.9 .
```

## run docker image using docker
```
docker run -ti \
-e IRODS_AUTHENTICATION_SCHEME=native -e IRODS_HOST=$IRODS_HOST -e IRODS_PORT=$IRODS_PORT \
-e IRODS_ZONE_NAME=$IRODS_ZONE_NAME -e IRODS_USER_NAME=$IRODS_USER_NAME -e IRODS_USER_PASSWORD=$IRODS_USER_PASSWORD \
cyverse/gocmd ls
```

## run docker image using docker-compose
```
docker compose run gocmd ls
```