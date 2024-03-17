#!/bin/bash
set -x

function cleanup {
docker container stop example
docker container rm example
docker network rm restNet
}

cleanup

docker build -t rest-app/example:v0.0.0 .

docker network inspect restNet
if [[ $? != 0 ]]; then
    docker network create restNet -d bridge
fi
docker run -d --hostname example --name example --network restNet rest-app/example:v0.0.0

# Give time for the container to start.
sleep 5
EXAMPLE_IP=$(docker inspect -f '{{.NetworkSettings.Networks.restNet.IPAddress}}' example)

echo -e "\n\nget admin token"
TOKEN_ADMIN=$(curl -k -s -X PUT -d '{"Email":"admin", "Password":"P@ss!234"}' \
    "https://$EXAMPLE_IP:8000/auth/login/")
echo $TOKEN_ADMIN

HTTP_STATUS=$(curl -k -s -w "\n|HTTP_STATUS=%{http_code}|\n" \
    -H "Authorization: Bearer $TOKEN_ADMIN" \
    "https://$EXAMPLE_IP:8000/" | \
    grep HTTP_STATUS | grep -o -E [0-9]*)
if [[ $HTTP_STATUS != 200 ]]; then
    echo "user auth was not accepted on root path"
    exitOnError
fi

echo -e "\n\nGET the root"
curl -k -s -H "Authorization: Bearer $TOKEN_ADMIN" "https://$PROD1_IP:8000/"

cleanup
