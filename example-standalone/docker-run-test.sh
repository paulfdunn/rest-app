#!/bin/bash
# docker-run-test.sh will run the example-standalone app in a docker container, get a token
# then GET the root path, resulting in:
# `hostname: example-standalone, rest-app - from github.com/paulfdunn/rest-app`
set -x

function exitOnError {
    echo "FAILED: $ME"
    cleanup
    exit $HTTP_STATUS
}

function cleanup {
docker container stop example-standalone
docker container rm example-standalone
}

ME=`basename $0`
echo "STARTING: $ME"

echo -e "\n\ncleanup prior to start"
cleanup

docker build -t rest-app/example-standalone:v0.0.0 .

docker run -p 127.0.0.1:8000:8000/tcp -d --hostname example-standalone --name example-standalone rest-app/example:v0.0.0

# Give time for the container to start.
sleep 5

echo -e "\n\nget admin token"
TOKEN_ADMIN=$(curl -k -s -X PUT -d '{"Email":"admin", "Password":"P@ss!234"}' \
    "https://127.0.0.1:8000/auth/login/")
echo $TOKEN_ADMIN

HTTP_STATUS=$(curl -k -s -w "\n|HTTP_STATUS=%{http_code}|\n" \
    -H "Authorization: Bearer $TOKEN_ADMIN" \
    "https://127.0.0.1:8000/" | \
    grep HTTP_STATUS | grep -o -E [0-9]*)
if [[ $HTTP_STATUS != 200 ]]; then
    echo "user auth was not accepted on root path"
    exitOnError
fi

echo -e "\n\n Root path requires auth. Try root path with no auth and get a 401."
HTTP_STATUS=$(curl -k -s -w "\n|HTTP_STATUS=%{http_code}|\n" \
     https://127.0.0.1:8000/ | \
    grep HTTP_STATUS | grep -o -E [0-9]*)
if [[ $HTTP_STATUS != 401 ]]; then
    echo "user auth was not required on root path"
    exitOnError
fi

echo -e "\n\n Get the root path using the admin token and get a 200."
HTTP_STATUS=$(curl -k -s -w "\n|HTTP_STATUS=%{http_code}|\n" \
    -H "Authorization: Bearer $TOKEN_ADMIN" \
    https://127.0.0.1:8000/ | \
    grep HTTP_STATUS | grep -o -E [0-9]*)
if [[ $HTTP_STATUS != 200 ]]; then
    echo "user auth was not accepted on root path"
    exitOnError
fi


echo -e "\n\ncleanup and exit"
ls -al
cleanup

echo "PASSED: $ME"