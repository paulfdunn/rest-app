# rest-app
This is the framework for a GO (GOLANG) based ReST API. This can be used as the basis for a GO based app, needing JWT user authentication, with logging and key/value store (KVS). 

There are multiple  parts to this repo:
* github.com/paulfdunn/rest-app/core - Application initialization and configuration. This is provided in a core package to allow leveraging many apps from the same configuration/initialization code. 
* github.com/paulfdunn/rest-app/example-standalone is provided to show an example of one service, using the core configuration/initialization provided, with authentication running as part of the service.

Key features:
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
See github.com/paulfdunn/rest-app/example-standalone for a full example and working application that provides a ReST API with JWT authentication.
* Run test-example-standalone.sh to build/run the example ReST API, authenticate, and issue a command
that passes a token for authentication.
example-standalone.go
* Calls ConfigInit to initialize the application configuration.
    * flag.Parse() is called; applicaitons should not call flag.Parse() as flag.Parse() can only be called once per application.
    * Optional - call config.Get() to merge in any saved configuration, which is modified by applications at runtime by calling config.Set().
* Calls OtherInit to initialize any other provided functionality.
* Calls blocking function ListenAndServeTLS to start serving your API.

## Usage - authentication as a standalone service, used by one or more independent services 
See github.com/paulfdunn/rest-app/example-auth-as-a-service for a full example and working application that provides authentication as a service (for use by one or more application services) and an example application service.
* Run test-example-auth-as-service.sh to build/run the example ReST API, authenticate via the authentication service, then issue a command to the application which validates the provided token.
* github.com/paulfdunn/rest-app/example-standalone is used to provide authentication, and github.com/paulfdunn/rest-app/example-auth-as-a-service is the application service.
* In this authentication model, tokens are issues by the authentication service using a relatively short expiration interval, and application services can only validate that a token was valid when issued. Application services cannot verify the user hasn't used the authentication service to log out or invalidate all tokens. Thus keeping a short JWTAuthExpirationInterval, and frequent refresh, is important.
