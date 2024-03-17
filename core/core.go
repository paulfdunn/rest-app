// Package core provides core functionality used across any apps you create with rest-app.
// Call ConfigInit to initialize the application configuration, OtherInit to initialize
// any other provided functionality, then call blocking function ListenAndServeTLS
// to start serving your API.
package core

import (
	"log"
	"net/http"
	"time"

	"github.com/paulfdunn/authJWT"
	"github.com/paulfdunn/go-helper/logh"
	"github.com/paulfdunn/go-helper/osh/runtimeh"
	"github.com/paulfdunn/rest-app/core/config"
)

// Add defaults can be changed  prior to calling ConfigInit
var (
	CheckLogSize      = 100
	MaxLogSize        = int64(100e6)
	CheckLogSizeAudit = 100
	MaxLogSizeAudit   = int64(2e6)
)

// ConfigInit initializes the configuration. It is separate from OtherInit as some configuration
// may be required prior to calling other Init functions.
func ConfigInit(cnfg config.Config, filepathsToDeleteOnReset []string) {
	config.Init(cnfg, CheckLogSize, MaxLogSize, CheckLogSizeAudit, MaxLogSizeAudit,
		filepathsToDeleteOnReset)
}

// OtherInit calls all required Init functions. Note that authentication is entirely optional.
func OtherInit(authConfig *authJWT.Config, mux *http.ServeMux, initialCred *authJWT.Credential) {
	if authConfig != nil {
		authJWT.Init(*authConfig, mux)
	}

	if initialCred != nil {
		if err := initialCred.AuthCreate(); err != nil {
			log.Fatalf("fatal: %s creating default account, error: %v", runtimeh.SourceInfo(), err)
		}
	}
}

// ListenAndServeTLS IS A BLOCKING FUNCTION that starts the HTTP server.
func ListenAndServeTLS(logName string, mux *http.ServeMux, port string, readTimeout time.Duration, writeTimeout time.Duration,
	certFilepath string, keyFilepath string) {
	server := &http.Server{
		Addr:           port,
		Handler:        mux,
		ReadTimeout:    readTimeout,
		WriteTimeout:   writeTimeout,
		MaxHeaderBytes: 1 << 20,
	}

	logh.Map[logName].Printf(logh.Error, "ListenAndServeTLS error: %v",
		server.ListenAndServeTLS(certFilepath, keyFilepath))
}
