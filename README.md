# rest-app
This is the framework for a GO (GOLANG) based ReST API. This can be used as the basis for a GO based app, needing JWT user authentication, with logging and key/value store (KVS). 

There are 2 parts to this application"
* github.com/paulfdunn/rest-app/core - Application initialization and configuration. This is provided in a core package to allow leveraging many apps from the same configuration/initialization code. 
* github.com/paulfdunn/example is provided to show an example of one app, using the core configuration/initialization provided.

Key features:
* Leveled logging; provided by github.com/paulfdunn/go-helper/logh 
* Key/value store (KVS); provided by github.com/paulfdunn/go-helper/databaseh. The KVS is used to store application configuration data and authentication data, but can be used for any other purpose as well.
* The KVS implements object serialization/deserialization, making it easy to persist objects. 
* Authentication is handled using JWT (JSON Web Tokens).
* Authentication supports 2 models: anyone can create a login, or only a registered user can create a new login. The later is the default in the example app.
* Authentication supports REGEX based validation/rules for passwords.

## Requirements
You must have GO installed. This code was build and tested against GO 121.7

Example of curl commands against the provided app, showing creating and deleting an auth (user).
```
pauldunn@PAULs-14-MBP example % ./readme-example-test.sh
++ basename ./readme-example-test.sh
+ ME=readme-example-test.sh
+ echo 'STARTING: readme-example-test.sh'
STARTING: readme-example-test.sh
+ echo -e '\n\ncleanup prior to start'


cleanup prior to start
+ cleanup
+ killall example
No matching processes belonging to you were found
+ rm example
rm: example: No such file or directory
+ rm example.db
rm: example.db: No such file or directory
+ rm 'example.log.*'
rm: example.log.*: No such file or directory
+ echo -e '\n\nbuild and run'


build and run
+ go build example.go
+ [[ 0 != 0 ]]
+ sleep 5
+ ./example -https-port=8000 -log-level=0 -log-filepath=./example.log
+ echo -e '\n\n Get admin token'


 Get admin token
++ curl -k -s -X PUT -d '{"Email":"admin", "Password":"P@ss!234"}' https://127.0.0.1:8000/auth/login/
+ TOKEN_ADMIN=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE3MTA2Mjc2MTYsImlzcyI6ImV4YW1wbGUiLCJFbWFpbCI6ImFkbWluIiwiVG9rZW5JRCI6IjcxMmQ1ZjlhLWJmNjMtNjZiNi01ZGMyLTE4MTE0ZTg0ODZjZCJ9.XZZfEebZBtn2o-EoHH6ENTN-rtv5-Mi_R22osRQL1BI
+ echo eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE3MTA2Mjc2MTYsImlzcyI6ImV4YW1wbGUiLCJFbWFpbCI6ImFkbWluIiwiVG9rZW5JRCI6IjcxMmQ1ZjlhLWJmNjMtNjZiNi01ZGMyLTE4MTE0ZTg0ODZjZCJ9.XZZfEebZBtn2o-EoHH6ENTN-rtv5-Mi_R22osRQL1BI
eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE3MTA2Mjc2MTYsImlzcyI6ImV4YW1wbGUiLCJFbWFpbCI6ImFkbWluIiwiVG9rZW5JRCI6IjcxMmQ1ZjlhLWJmNjMtNjZiNi01ZGMyLTE4MTE0ZTg0ODZjZCJ9.XZZfEebZBtn2o-EoHH6ENTN-rtv5-Mi_R22osRQL1BI
+ echo -e '\n\n Root path requires auth. Try root path with no auth and get a 401.'


 Root path requires auth. Try root path with no auth and get a 401.
++ curl -k -s -w '\n|HTTP_STATUS=%{http_code}|\n' https://127.0.0.1:8000/
++ grep HTTP_STATUS
++ grep -o -E '[0-9]*'
+ HTTP_STATUS=401
+ [[ 401 != 401 ]]
+ echo -e '\n\n Try again providing the token and get a 200.'


 Try again providing the token and get a 200.
++ curl -k -s -w '\n|HTTP_STATUS=%{http_code}|\n' -H 'Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE3MTA2Mjc2MTYsImlzcyI6ImV4YW1wbGUiLCJFbWFpbCI6ImFkbWluIiwiVG9rZW5JRCI6IjcxMmQ1ZjlhLWJmNjMtNjZiNi01ZGMyLTE4MTE0ZTg0ODZjZCJ9.XZZfEebZBtn2o-EoHH6ENTN-rtv5-Mi_R22osRQL1BI' https://127.0.0.1:8000/
++ grep HTTP_STATUS
++ grep -o -E '[0-9]*'
+ HTTP_STATUS=200
+ [[ 200 != 200 ]]
+ echo -e '\n\n Create a user.'


 Create a user.
++ curl -k -s -w '\n|HTTP_STATUS=%{http_code}|\n' -d '{"Email":"user", "Password":"P@ss!234"}' -H 'Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE3MTA2Mjc2MTYsImlzcyI6ImV4YW1wbGUiLCJFbWFpbCI6ImFkbWluIiwiVG9rZW5JRCI6IjcxMmQ1ZjlhLWJmNjMtNjZiNi01ZGMyLTE4MTE0ZTg0ODZjZCJ9.XZZfEebZBtn2o-EoHH6ENTN-rtv5-Mi_R22osRQL1BI' https://127.0.0.1:8000/auth/create/
++ grep HTTP_STATUS
++ grep -o -E '[0-9]*'
+ HTTP_STATUS=201
+ [[ 201 != 201 ]]
+ echo -e '\n\n User logs in and deletes their own account'


 User logs in and deletes their own account
++ curl -k -s -X PUT -d '{"Email":"user", "Password":"P@ss!234"}' https://127.0.0.1:8000/auth/login/
+ TOKEN_USER=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE3MTA2Mjc2MTYsImlzcyI6ImV4YW1wbGUiLCJFbWFpbCI6InVzZXIiLCJUb2tlbklEIjoiNGMyNGNjNjYtNTAzYi1jNjJhLTA0MmQtNTU0NjNkYWMyNGMwIn0.L5GtsDZvoXJ4ydQ7hz_H8VKvORCVBhwHB6xApRSoYKc
++ curl -k -s -w '\n|HTTP_STATUS=%{http_code}|\n' -H 'Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE3MTA2Mjc2MTYsImlzcyI6ImV4YW1wbGUiLCJFbWFpbCI6InVzZXIiLCJUb2tlbklEIjoiNGMyNGNjNjYtNTAzYi1jNjJhLTA0MmQtNTU0NjNkYWMyNGMwIn0.L5GtsDZvoXJ4ydQ7hz_H8VKvORCVBhwHB6xApRSoYKc' -X DELETE https://127.0.0.1:8000/auth/delete/
++ grep HTTP_STATUS
++ grep -o -E '[0-9]*'
+ HTTP_STATUS=204
+ [[ 204 != 204 ]]
+ echo -e '\n\ncleanup and exit'


cleanup and exit
+ cleanup
+ killall example
./readme-example-test.sh: line 10: 51486 Terminated: 15          ./example -https-port=8000 -log-level=0 -log-filepath=./example.log
+ rm example
+ rm example.db
+ rm example.log.0 example.log.audit.0
+ echo 'PASSED: readme-example-test.sh'
PASSED: readme-example-test.sh
```

And see what is in the logs:
```
paulfdunn@penguin:~/go/src/github.com/paulfdunn/example$ cat example.log.0
pauldunn@PAULs-14-MBP example % cat example.log.0
2024/03/16 22:07:40.160498 config.go:115:    info: example is starting....
2024/03/16 22:07:40.160837 config.go:117:    info: logFilepath:./example.log
2024/03/16 22:07:40.160853 config.go:118:    info: auditLogFilepath:./example.log.audit
2024/03/16 22:07:40.164247 example.go:65:    info: Config: {"HTTPSPort":8000,"LogFilepath":"./example.log","LogLevel":0,"AppName":"example","AppPath":"/Users/pauldunn/go/src/github.com/paulfdunn/rest-app/example","AuditLogName":"example.audit","DataSourceName":"/Users/pauldunn/go/src/github.com/paulfdunn/rest-app/example/example.db","JWTKeyFilepath":"/Users/pauldunn/go/src/github.com/paulfdunn/rest-app/example/key/jwt.rsa.public","JWTAuthRemoveInterval":60000000000,"JWTAuthTimeoutInterval":900000000000,"LogName":"example","NewDataSource":true,"PasswordValidation":["^[\\S]{8,32}$","[a-z]","[A-Z]","[!#$%'()*+,-.\\\\/:;=?@\\[\\]^_{|}~]","[0-9]"],"PersistentDirectory":"/Users/pauldunn/go/src/github.com/paulfdunn/rest-app/example"}
2024/03/16 22:07:40.164598 authJWT.go:112:    info: Registered handler: /auth/create/
2024/03/16 22:07:40.164656 authJWT.go:115:    info: Registered handler: /auth/delete/
2024/03/16 22:07:40.164668 authJWT.go:118:    info: Registered handler: /auth/info/
2024/03/16 22:07:40.164676 authJWT.go:121:    info: Registered handler: /auth/login/
2024/03/16 22:07:40.164686 authJWT.go:124:    info: Registered handler: /auth/logout/
2024/03/16 22:07:40.164695 authJWT.go:127:    info: Registered handler: /auth/logout-all/
2024/03/16 22:07:40.164703 authJWT.go:130:    info: Registered handler: /auth/refresh/
2024/03/16 22:07:40.247841 example.go:112:    info: Created default auth: admin
2024/03/16 22:07:40.247869 example.go:94:    info: Registered handler: /
2024/03/16 22:07:44.878212 example.go:116:    info: rest-app handler {GET / HTTP/2.0 2 0 map[Accept:[*/*] Authorization:[Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE3MTA2Mjc3NjQsImlzcyI6ImV4YW1wbGUiLCJFbWFpbCI6ImFkbWluIiwiVG9rZW5JRCI6ImU2ZDVkYmUzLTJiY2EtZWUyNy0yMTBhLTViNjBmMzM2OGQyMiJ9.N-Dm0nRStAL2pRoCPBH1iAT2Bi5AZtELL5-uRNPE2Wc] User-Agent:[curl/8.4.0]] 0x14000274c00 <nil> 0 [] false 127.0.0.1:8000 map[] map[] <nil> map[] 127.0.0.1:65067 / 0x1400030e0b0 <nil> <nil> 0x140000f0c80}

pauldunn@PAULs-14-MBP example % cat example.log.audit.0
2024/03/16 22:07:40.160756 config.go:116:   audit: example is starting....
2024/03/16 22:07:44.964637 handlers.go:50:   audit: status: 201| req:&{Method:POST URL:/auth/create/ Proto:HTTP/2.0 ProtoMajor:2 ProtoMinor:0 Header:map[Accept:[*/*] Authorization:[Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE3MTA2Mjc3NjQsImlzcyI6ImV4YW1wbGUiLCJFbWFpbCI6ImFkbWluIiwiVG9rZW5JRCI6ImU2ZDVkYmUzLTJiY2EtZWUyNy0yMTBhLTViNjBmMzM2OGQyMiJ9.N-Dm0nRStAL2pRoCPBH1iAT2Bi5AZtELL5-uRNPE2Wc] Content-Length:[39] Content-Type:[application/x-www-form-urlencoded] User-Agent:[curl/8.4.0]] Body:0x140001971a0 GetBody:<nil> ContentLength:39 TransferEncoding:[] Close:false Host:127.0.0.1:8000 Form:map[] PostForm:map[] MultipartForm:<nil> Trailer:map[] RemoteAddr:127.0.0.1:65068 RequestURI:/auth/create/ TLS:0x1400030e210 Cancel:<nil> Response:<nil> ctx:0x1400019a780}| body: body not logged, contains credentials for user|

2024/03/16 22:07:45.068146 handlers.go:50:   audit: status: 204| req:&{Method:DELETE URL:/auth/delete/ Proto:HTTP/2.0 ProtoMajor:2 ProtoMinor:0 Header:map[Accept:[*/*] Authorization:[Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE3MTA2Mjc3NjUsImlzcyI6ImV4YW1wbGUiLCJFbWFpbCI6InVzZXIiLCJUb2tlbklEIjoiODJkNTU2MGEtNzQ3ZS04Mjk3LTNlZmUtYTQ0ZTgyMDFhYjM0In0.wCB3FfoD0llo3-8ijIZLbXuAymuGemPq3VLhlAm4zJc] User-Agent:[curl/8.4.0]] Body:0x140001024e0 GetBody:<nil> ContentLength:0 TransferEncoding:[] Close:false Host:127.0.0.1:8000 Form:map[] PostForm:map[] MultipartForm:<nil> Trailer:map[] RemoteAddr:127.0.0.1:65070 RequestURI:/auth/delete/ TLS:0x140003d60b0 Cancel:<nil> Response:<nil> ctx:0x14000100190}| body: |
```