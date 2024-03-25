#!/bin/bash
# This script will build and run the example-auth-as-service app, then issue some curl
# commands to the API.
set -x

function exitOnError {
    echo "FAILED: $ME"
    cleanup
    exit $HTTP_STATUS
}

function cleanup {
    killall example-auth-as-service
    killall example-standalone
    rm example-auth-as-service
    rm example-auth-as-service.config.db
    rm example-auth-as-service.log.*
    rm ../example-standalone/example-standalone
    rm ../example-standalone/example-standalone.config.db
    rm ../example-standalone/example-standalone.auth.db
    rm ../example-standalone/example-standalone.log.*
}

ME=`basename $0`
echo "STARTING: $ME"

echo -e "\n\ncleanup prior to start"
cleanup

echo -e "\n\nbuild and run"
cd ../example-standalone
go build example-standalone.go
if [[ $? != 0 ]]; then
    echo "FAILED: go build failed"
    exit
fi
cd ../example-auth-as-service
go build example-auth-as-service.go
if [[ $? != 0 ]]; then
    echo "FAILED: go build failed"
    exit
fi
# Run the apps in the background.
../example-standalone/example-standalone  -https-port=8000 -log-level=0 -log-filepath=../example/example-standalone/example-standalone.log &
./example-auth-as-service  -https-port=8001 -log-level=0 -log-filepath=./example-auth-as-service.log &
# Wait for apps to start.
sleep 5

echo -e "\n\n Get admin token from the authentication service"
TOKEN_ADMIN=$(curl -k -s -X PUT -d '{"Email":"admin", "Password":"P@ss!234"}' \
    https://127.0.0.1:8000/auth/login/)
echo $TOKEN_ADMIN

echo -e "\n\n Root path requires auth. Try root path with no auth and get a 401."
HTTP_STATUS=$(curl -k -s -w "\n|HTTP_STATUS=%{http_code}|\n" \
     https://127.0.0.1:8001/ | \
    grep HTTP_STATUS | grep -o -E [0-9]*)
if [[ $HTTP_STATUS != 401 ]]; then
    echo "user auth was not required on root path"
    exitOnError
fi

echo -e "\n\n Get the root path using the admin token and get a 200."
HTTP_STATUS=$(curl -k -s -w "\n|HTTP_STATUS=%{http_code}|\n" \
    -H "Authorization: Bearer $TOKEN_ADMIN" \
    https://127.0.0.1:8001/ | \
    grep HTTP_STATUS | grep -o -E [0-9]*)
if [[ $HTTP_STATUS != 200 ]]; then
    echo "user auth was not accepted on root path"
    exitOnError
fi

echo -e "\n\n"
cat example-auth-as-service.log.0
echo -e "\n\n"
cat example-auth-as-service.log.audit.0

echo -e "\n\ncleanup and exit"
ls -al
cleanup

echo "PASSED: $ME"