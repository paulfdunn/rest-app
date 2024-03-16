module github.com/paulfdunn/example

go 1.21.7

replace (
	github.com/paulfdunn/core => /Users/pauldunn/go/src/github.com/paulfdunn/rest-app/core
	github.com/paulfdunn/core/config => /Users/pauldunn/go/src/github.com/paulfdunn/rest-app/core/config
)

require (
	github.com/paulfdunn/authJWT v1.0.3
	github.com/paulfdunn/go-helper/logh v1.0.7
	github.com/paulfdunn/go-helper/osh v1.0.10
	github.com/paulfdunn/core v1.0.0
	github.com/paulfdunn/core/config v0.0.0-00010101000000-000000000000
)

require (
	github.com/dgrijalva/jwt-go v3.2.0+incompatible // indirect
	github.com/mattn/go-sqlite3 v1.14.22 // indirect
	github.com/paulfdunn/go-helper/databaseh v1.0.7 // indirect
	github.com/paulfdunn/go-helper/neth v1.0.7 // indirect
	golang.org/x/crypto v0.21.0 // indirect
)
