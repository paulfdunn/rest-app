#!/bin/bash
set -x

function exitOnError {
    echo "FAILED: $ME"
    cleanup
    exit $HTTP_STATUS
}

function cleanup {
    killall example
    rm example
    rm example.db
    rm example.log.*
}

ME=`basename $0`
echo "STARTING: $ME"

echo -e "\n\ncleanup prior to start"
cleanup

echo -e "\n\nbuild and run"
go build example.go
if [[ $? != 0 ]]; then
    echo "FAILED: go build failed"
    exit
fi
./example  -https-port=8000 -log-level=0 -log-filepath=./example.log &
# wait for app to start
sleep 5

echo -e "\n\n Get admin token"
TOKEN_ADMIN=$(curl -k -s -X PUT -d '{"Email":"admin", "Password":"P@ss!234"}' \
    https://127.0.0.1:8000/auth/login/)
echo $TOKEN_ADMIN

echo -e "\n\n Root path requires auth. Try root path with no auth and get a 401."
HTTP_STATUS=$(curl -k -s -w "\n|HTTP_STATUS=%{http_code}|\n" \
     https://127.0.0.1:8000/ | \
    grep HTTP_STATUS | grep -o -E [0-9]*)
if [[ $HTTP_STATUS != 401 ]]; then
    echo "user auth was not required on root path"
    exitOnError
fi

echo -e "\n\n Try again providing the token and get a 200."
HTTP_STATUS=$(curl -k -s -w "\n|HTTP_STATUS=%{http_code}|\n" \
    -H "Authorization: Bearer $TOKEN_ADMIN" \
    https://127.0.0.1:8000/ | \
    grep HTTP_STATUS | grep -o -E [0-9]*)
if [[ $HTTP_STATUS != 200 ]]; then
    echo "user auth was not accepted on root path"
    exitOnError
fi

echo -e "\n\n Create a user."
HTTP_STATUS=$(curl -k -s -w "\n|HTTP_STATUS=%{http_code}|\n" -d '{"Email":"user", "Password":"P@ss!234"}'\
    -H "Authorization: Bearer $TOKEN_ADMIN" \
    https://127.0.0.1:8000/auth/create/ | \
    grep HTTP_STATUS | grep -o -E [0-9]*)
if [[ $HTTP_STATUS != 201 ]]; then
    echo "user create failed"
    exitOnError
fi

echo -e "\n\n User logs in and deletes their own account"
TOKEN_USER=$(curl -k -s -X PUT -d '{"Email":"user", "Password":"P@ss!234"}' \
    https://127.0.0.1:8000/auth/login/)
HTTP_STATUS=$(curl -k -s -w "\n|HTTP_STATUS=%{http_code}|\n" \
    -H "Authorization: Bearer $TOKEN_USER" \
    -X DELETE \
    https://127.0.0.1:8000/auth/delete/ | \
    grep HTTP_STATUS | grep -o -E [0-9]*)
if [[ $HTTP_STATUS != 204 ]]; then
    echo "user delete failed"
    exitOnError
fi

echo -e "\n\ncleanup and exit"
# cleanup

echo "PASSED: $ME"