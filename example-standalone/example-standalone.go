// example-standalone is an example of using github.com/paulfdunn/rest-app (a framework for a
// GO (GOLANG) based ReST APIs) to create a standalone application that
// uses github.com/paulfdunn/authJWT for authentication.
// This application includes the authentication directly in the service.
package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"

	"github.com/paulfdunn/authJWT"
	"github.com/paulfdunn/go-helper/logh"
	"github.com/paulfdunn/go-helper/osh/runtimeh"
	"github.com/paulfdunn/rest-app/core"
	"github.com/paulfdunn/rest-app/core/config"
)

const (
	authFileSuffix = ".auth.db"
	// relative file paths will be joined with appPath to create the path to the file.
	relativeCertFilePath   = "/key/rest-app.crt"
	relativeKeyFilePath    = "/key/rest-app.key"
	relativePrivateKeyPath = "/key/jwt.rsa.private"
	relativePublicKeyPath  = "../example-standalone/key/jwt.rsa.public"
)

var (
	appName = "example-standalone"

	// API timeouts
	apiReadTimeout  = 10 * time.Second
	apiWriteTimeout = 10 * time.Second

	// Any files in this list will be deleted on application reset using the CLI parameter
	// See core/config.Init
	filepathsToDeleteOnReset = []string{}

	// logh function pointers make logging calls more compact, but are optional.
	lp  func(level logh.LoghLevel, v ...interface{})
	lpf func(level logh.LoghLevel, format string, v ...interface{})

	// initial credentials
	initialEmail    = "admin"
	initialPassword = "P@ss!234"
)

func main() {
	defer func() {
		if err := recover(); err != nil {
			errOut := fmt.Sprintf("panic: %+v\n%+v", err, string(debug.Stack()))
			fmt.Println(errOut)
			lp(logh.Error, errOut)
			err = logh.ShutdownAll()
			if err != nil {
				fmt.Printf("logh.ShutdownAll error:%+v", errOut)
			}
		}
	}()

	// flag.Parse() is called by config.Config; apps should not call flag.Parse()
	inputConfig := config.Config{AppName: &appName, LogName: &appName}

	// default to the executable path.
	exe, err := os.Executable()
	if err != nil {
		log.Fatalf("fatal: %s fatal: Could not find executable path.", runtimeh.SourceInfo())
	}
	appPath := filepath.Dir(exe)

	// Create the default config, then read overwrite any config that might have been saved at
	// runtime (from a previous run, using config.Set()) with a call to config.Get()
	core.ConfigInit(inputConfig, filepathsToDeleteOnReset)
	// logh function pointers make logging calls more compact, but are optional. Must be done after config
	// has initialized logging
	lp = logh.Map[appName].Println
	lpf = logh.Map[appName].Printf
	var runtimeConfig config.Config
	if runtimeConfig, err = config.Get(); err != nil {
		log.Fatalf("fatal: %s getting running config, error:%v", runtimeh.SourceInfo(), err)
	}
	lpf(logh.Info, "Config: %s", runtimeConfig)

	privateKeyPath := filepath.Join(appPath, relativePrivateKeyPath)
	publicKeyPath := filepath.Join(appPath, relativePublicKeyPath)
	jwtRemovalInterval := time.Minute
	jwtExpirationInterval := time.Minute * 15
	// Technically the authJWT.Config could be embedded in the core.Config, but that opens
	// security holes allowing someone to redirect authentication to a different source.
	ac := authJWT.Config{
		AppName:                   *runtimeConfig.AppName,
		AuditLogName:              *runtimeConfig.AuditLogName,
		DataSourcePath:            filepath.Join(filepath.Dir(*runtimeConfig.DataSourcePath), *runtimeConfig.AppName+authFileSuffix),
		CreateRequiresAuth:        true,
		JWTAuthRemoveInterval:     jwtRemovalInterval,
		JWTAuthExpirationInterval: jwtExpirationInterval,
		JWTPrivateKeyPath:         privateKeyPath,
		JWTPublicKeyPath:          publicKeyPath,
		LogName:                   *runtimeConfig.LogName,
	}
	mux := http.NewServeMux()
	var initialCreds *authJWT.Credential
	if *runtimeConfig.DataSourceIsNew {
		initialCreds = &authJWT.Credential{Email: &initialEmail, Password: &initialPassword}
	}
	core.OtherInit(&ac, mux, initialCreds)

	// Registering with the trailing slash means the naked path is redirected to this path.
	path := "/"
	mux.HandleFunc(path, authJWT.HandlerFuncAuthJWTWrapper(handler))
	lpf(logh.Info, "Registered handler: %s\n", path)

	cfp := filepath.Join(appPath, relativeCertFilePath)
	kfp := filepath.Join(appPath, relativeKeyFilePath)
	// blocking call
	core.ListenAndServeTLS(appName, mux, fmt.Sprintf(":%d", *runtimeConfig.HTTPSPort),
		apiReadTimeout, apiWriteTimeout, cfp, kfp)
}

// handler does nothing - it is just an example of creating a handler and is used by the example
// script so there is an authenticated endpoint to hit.
func handler(w http.ResponseWriter, r *http.Request) {
	lpf(logh.Info, "handler http.request: %v\n", *r)
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
		lpf(logh.Error, "hostname error: %v\n", err)
	}
	_, err = w.Write([]byte(fmt.Sprintf("hostname: %s, rest-app - from github.com/paulfdunn/rest-app", hostname)))
	if err != nil {
		lpf(logh.Error, "handler error: %v\n", err)
	}
}
