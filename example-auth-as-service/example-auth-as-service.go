// example-auth-as-service is an example of using github.com/paulfdunn/rest-app (a framework for a
// GO (GOLANG) based ReST APIs) to create a service that does not include authentication, but
// relies on a separate auth service to provide clients with authentication service. This service
// is then called with a token provisioned from the separate authentication service, and this
// service validates the token for paths requiring authentication.
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
	// relative file paths will be joined with appPath to create the path to the file.
	relativeCertFilePath   = "/key/rest-app.crt"
	relativeKeyFilePath    = "/key/rest-app.key"
	relativePrivateKeyPath = "../example-standalone/key/jwt.rsa.private"
)

var (
	appName = "example-auth-as-service"

	// API timeouts
	apiReadTimeout  = 10 * time.Second
	apiWriteTimeout = 10 * time.Second

	// Any files in this list will be deleted on application reset using the CLI parameter
	// See core/config.Init
	filepathsToDeleteOnReset = []string{}

	// logh function pointers make logging calls more compact, but are optional.
	lp  func(level logh.LoghLevel, v ...interface{})
	lpf func(level logh.LoghLevel, format string, v ...interface{})
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

	// logh function pointers make logging calls more compact, but are optional.
	lp = logh.Map[appName].Println
	lpf = logh.Map[appName].Printf

	// default to the executable path.
	exe, err := os.Executable()
	if err != nil {
		log.Fatalf("fatal: %s fatal: Could not find executable path.", runtimeh.SourceInfo())
	}
	appPath := filepath.Dir(exe)

	// Create the default config, then read overwrite any config that might have been saved at
	// runtime (from a previous run, using config.Set()) with a call to config.Get()
	core.ConfigInit(inputConfig, filepathsToDeleteOnReset)
	var runtimConfig config.Config
	if runtimConfig, err = config.Get(); err != nil {
		log.Fatalf("fatal: %s getting running config, error:%v", runtimeh.SourceInfo(), err)
	}
	lpf(logh.Info, "Config: %s", runtimConfig)

	privateKeyPath := filepath.Join(appPath, relativePrivateKeyPath)
	ac := authJWT.Config{
		AppName:    *runtimConfig.AppName,
		JWTKeyPath: privateKeyPath,
		LogName:    *runtimConfig.LogName,
	}
	mux := http.NewServeMux()
	core.OtherInit(&ac, nil, nil)

	// Registering with the trailing slash means the naked path is redirected to this path.
	path := "/"
	mux.HandleFunc(path, authJWT.HandlerFuncAuthJWTWrapper(handler))
	lpf(logh.Info, "Registered handler: %s\n", path)

	cfp := filepath.Join(appPath, relativeCertFilePath)
	kfp := filepath.Join(appPath, relativeKeyFilePath)
	// blocking call
	core.ListenAndServeTLS(appName, mux, fmt.Sprintf(":%d", *runtimConfig.HTTPSPort),
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
