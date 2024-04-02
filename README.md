# rest-app
This is the framework for a GO (GOLANG) based ReST API. This can be used as the basis for a GO based app, needing JWT user authentication, with logging and key/value store (KVS). 

There are multiple  parts to this repo:
* github.com/paulfdunn/rest-app/core - Application initialization and configuration. This is provided in a core package to allow leveraging many apps from the same configuration/initialization code. 
* github.com/paulfdunn/rest-app/example-auth-as-service is provided to show an example of using the core configuration/initialization to create an authentication service. This service is designed to be called by other services that have access to the public key of the anthentication service in order to decode the JWT tokens provided by the authentication service.
    * github.com/paulfdunn/rest-app/example-auth-as-service can also be used as the basis for a standalone service that has JWT based authentication by creating additional endpoint handlers and calling mux.HandleFunc with those functions.
*  github.com/paulfdunn/rest-app/example-telemetry is another service that uses the core configuration/initialization, but is also uses github.com/paulfdunn/rest-app/example-auth-as-service for authentication. The purpose of example-telemetry is to have a ReST API that allows callers to execute commands, have that output (STDOUT/STDERR) saved to files, then download all output in a ZIP file. (This service is designed for internal use in infrastructure environments, embedded applicaitons, etc. You should never allow anyone to execute arbitrary commands on your servers.)

Key features of rest-app:
* Leveled logging; provided by github.com/paulfdunn/go-helper/logh 
* Key/value store (KVS); provided by github.com/paulfdunn/go-helper/databaseh. The KVS is used to store application configuration data and authentication data, but can be used for any other purpose as well.
    * The configuration can be changed dynamically, persisted, and will be re-loaded when the application restarts.
* The KVS implements object serialization/deserialization, making it easy to persist objects. 
* Authentication (optional) is handled using JWT (JSON Web Tokens).
    * Authentication supports 2 models: anyone can create a login, or only a registered user can create a new login. The later is the default in the example app.
    * Authentication supports REGEX based validation/rules for passwords.
    * All authentication data is kept in a datastore separate from application configuration. 
    * Authentication can be embedded in a service, or a standalone service.

## Usage - standalone service with authentication
See github.com/paulfdunn/rest-app/example-auth-as-service for a full example and working application that provides a ReST API with JWT authentication.
* Run test-example-auth-as-service.sh to build/run the example ReST API, authenticate, and issue a command
that passes a token for authentication.
example-auth-as-service.go
* Calls ConfigInit to initialize the application configuration.
    * flag.Parse() is called; applicaitons should not call flag.Parse() as flag.Parse() can only be called once per application.
    * Optional - call config.Get() to merge in any saved configuration, which is modified by applications at runtime by calling config.Set().
* Calls OtherInit to initialize any other provided functionality.
* Calls blocking function ListenAndServeTLS to start serving your API.

## Usage - telemetry
See github.com/paulfdunn/rest-app/example-telemetry. 
* Run test-example-telemetry.sh to: build and run both the authentication and telemetry services, get a JWT token from the auth service, then make telemetry requests.