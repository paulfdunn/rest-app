// example is an example of how to base multiple products off of the same init/config functionality,
// using rest-app (a framework for a GO (GOLANG) based ReST APIs).
// This can be used as the basis for a GO based app, needing JWT user authentication,
// with logging and key/value store (KVS).
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
	appName = "example"
)

var (
	// initial credentials
	initialEmail    = "admin"
	initialPassword = "P@ss!234"

	filepathsToDeleteOnReset = []string{}
)

func main() {
	defer func() {
		if err := recover(); err != nil {
			errOut := fmt.Sprintf("panic: %+v\n%+v", err, string(debug.Stack()))
			fmt.Println(errOut)
			logh.Map[appName].Println(logh.Error, errOut)
			logh.ShutdownAll()
		}
	}()

	an := appName
	inputConfig := config.Config{AppName: &an, LogName: &an}

	// default to the executable path.
	exe, err := os.Executable()
	if err != nil {
		log.Fatalf("fatal: %s fatal: Could not find executable path.", runtimeh.SourceInfo())
	}
	appPath := filepath.Dir(exe)

	privateKeyPath := filepath.Join(appPath, "/key/jwt.rsa.private")
	jwtRemovalInterval := time.Minute
	jwtExpirationInterval := time.Minute * 15
	core.ConfigInit(inputConfig, filepathsToDeleteOnReset)
	var runtimConfig config.Config
	if runtimConfig, err = config.Get(); err != nil {
		log.Fatalf("fatal: %s getting running config, error:%v", runtimeh.SourceInfo(), err)
	}
	logh.Map[appName].Printf(logh.Info, "Config: %s", runtimConfig)

	ac := authJWT.Config{
		AppName:                   *runtimConfig.AppName,
		AuditLogName:              *runtimConfig.AuditLogName,
		DataSourcePath:            *runtimConfig.DataSourcePath,
		CreateRequiresAuth:        true,
		JWTAuthRemoveInterval:     jwtRemovalInterval,
		JWTAuthExpirationInterval: jwtExpirationInterval,
		JWTKeyPath:                privateKeyPath,
		LogName:                   *runtimConfig.LogName,
	}
	mux := http.NewServeMux()
	var initialCreds *authJWT.Credential
	if *runtimConfig.DataSourceIsNew {
		initialCreds = &authJWT.Credential{Email: &initialEmail, Password: &initialPassword}
	}
	core.OtherInit(&ac, mux, initialCreds)

	// Registering with the trailing slash means the naked path is redirected to this path.
	path := "/"
	mux.HandleFunc(path, authJWT.HandlerFuncAuthJWTWrapper(handler))
	logh.Map[appName].Printf(logh.Info, "Registered handler: %s\n", path)

	// blocking call
	cfp := filepath.Join(appPath, "/key/rest-app.crt")
	kfp := filepath.Join(appPath, "/key/rest-app.key")
	core.ListenAndServeTLS(appName, mux, fmt.Sprintf(":%d", *runtimConfig.HTTPSPort),
		10*time.Second, 10*time.Second, cfp, kfp)
}

func handler(w http.ResponseWriter, r *http.Request) {
	logh.Map[appName].Printf(logh.Info, "rest-app handler %v\n", *r)
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
		logh.Map[appName].Printf(logh.Error, "hostname error: %v\n", err)
	}
	w.Write([]byte(fmt.Sprintf("hostname: %s, rest-app - from github.com/paulfdunn/rest-app", hostname)))
}
