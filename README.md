# rest-app
This is the framework for a GO (GOLANG) based ReST API. This can be used as the basis for a GO based app, needing JWT user authentication, with logging and key/value store (KVS). 

There are 2 parts to this application"
* github.com/paulfdunn/rest-app/core - Application initialization and configuration. This is provided in a core package to allow leveraging many apps from the same configuration/initialization code. 
* github.com/paulfdunn/rest-app/example is provided to show an example of one app, using the core configuration/initialization provided.

Key features:
* Leveled logging; provided by github.com/paulfdunn/go-helper/logh 
* Key/value store (KVS); provided by github.com/paulfdunn/go-helper/databaseh. The KVS is used to store application configuration data and authentication data, but can be used for any other purpose as well.
* The KVS implements object serialization/deserialization, making it easy to persist objects. 
* Authentication is handled using JWT (JSON Web Tokens).
* Authentication supports 2 models: anyone can create a login, or only a registered user can create a new login. The later is the default in the example app.
* Authentication supports REGEX based validation/rules for passwords.
