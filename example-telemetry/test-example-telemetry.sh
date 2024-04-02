#!/bin/bash
# This script will build and run example-auth-as-service for authentication, then build and run
# example-telemetry app, then issue some curl commands to the API.
# ReST note - you must terminate a URL with "/" or the request will redirect to the URL with
# a trailing "/" using a GET method.
# curl note - you must escape the "?" in a query string with a "\", otherwise the shell
# tries to expand the "?" and the shell returns an error "no matches found"
set -x

function exitOnError {
    echo "FAILED: $ME"
    cleanup
    exit $HTTP_STATUS
}

function cleanup {
    killall example-auth-as-service
    killall example-telemetry
    rm example-telemetry
    rm example-telemetry*.db
    rm example-telemetry.log.*
    rm example-auth-as-service
    rm example-auth-as-service*.db
    rm example-auth-as-service.log.*
    rm -rf ./taskdata
}

ME=`basename $0`
echo "STARTING: $ME"

echo -e "\n\ncleanup prior to start"
cleanup

echo -e "\n\nbuild and run"
cd ../example-auth-as-service
go build
if [[ $? != 0 ]]; then
    echo "FAILED: go build failed"
    exit
fi
cd ../example-telemetry
go build
if [[ $? != 0 ]]; then
    echo "FAILED: go build failed"
    exit
fi
# Run the apps in the background.
../example-auth-as-service/example-auth-as-service  -https-port=8000 -log-level=0 -log-filepath=./example-auth-as-service.log  -persistent-directory=./&
./example-telemetry  -https-port=8001 -log-level=0 -log-filepath=./example-telemetry.log -persistent-directory=./ &
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

echo -e "\n\n Create a task."
UUID=$(curl -k -s -d '{"Command":["ls -al"]}' \
    -H "Authorization: Bearer $TOKEN_ADMIN" \
    https://127.0.0.1:8001/task/ | jq -r '.UUID')
echo ${UUID}

echo -e "\n\n Get the task status"
curl -k -s  \
    -H "Authorization: Bearer $TOKEN_ADMIN" \
    https://127.0.0.1:8001/status/\?uuid=${UUID}

echo -e "\n\nDownload the file"
HTTP_STATUS=$(curl -k -s -X GET -w "\n|HTTP_STATUS=%{http_code}|\n" \
    -o ./taskdata/telemetry.zip \
    -H "Authorization: Bearer $TOKEN_ADMIN" \
    https://127.0.0.1:8001/task/\?uuid=${UUID} | \
    grep HTTP_STATUS | grep -o -E [0-9]*)
if [[ $HTTP_STATUS != 200 ]]; then
    echo "download failed"
    exitOnError
fi

echo -e "\n\n"
cat example-telemetry.log.0
echo -e "\n\n"
cat example-telemetry.log.audit.0

echo "Press any key to continue..."
# -s: Do not echo input coming
# -n 1: Read one character
read -s -n 1

echo -e "\n\ncleanup and exit"
cleanup
ls -al

echo "PASSED: $ME"