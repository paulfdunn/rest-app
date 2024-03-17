# rest-app
This is the framework for a GO (GOLANG) based ReST API. This can be used as the basis for a GO based app, needing JWT user authentication, with logging and key/value store (KVS). 

There are 2 parts to this application"
* github.com/paulfdunn/rest-app/core - Application initialization and configuration. This is provided in a core package to allow leveraging many apps from the same configuration/initialization code. 
* github.com/paulfdunn/rest-app/example is provided to show an example of one app, using the core configuration/initialization provided.

Key features:
* Leveled logging; provided by github.com/paulfdunn/go-helper/logh 
* Key/value store (KVS); provided by github.com/paulfdunn/go-helper/databaseh. The KVS is used to store application configuration data and authentication data, but can be used for any other purpose as well.
    * The configuration can be changed dynamically, persisted, and will be re-loaded when the application restarts.
* The KVS implements object serialization/deserialization, making it easy to persist objects. 
* Authentication (optional) is handled using JWT (JSON Web Tokens).
    * Authentication supports 2 models: anyone can create a login, or only a registered user can create a new login. The later is the default in the example app.
    * Authentication supports REGEX based validation/rules for passwords.
    * All authentication data is kept in a datastore separate from application configuration. 

## Usage
See github.com/paulfdunn/rest-app/example for a full example and working application.
* Call ConfigInit to initialize the application configuration.
    * Optional - call config.Get() to merge in any saved configuration, which is modified by applications at runtime by calling config.Set().
* Call OtherInit to initialize any other provided functionality.
* Call blocking function ListenAndServeTLS to start serving your API.