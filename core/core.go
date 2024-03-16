// Package core provides core functionality used across any apps you create with rest-app
package core

import (
	"net/http"
	"time"

	"github.com/paulfdunn/authJWT"
	"github.com/paulfdunn/go-helper/logh"
	"github.com/paulfdunn/rest-app/core/config"
)

const (
	checkLogSize      = 100
	maxLogSize        = int64(100e6)
	checkLogSizeAudit = 100
	maxLogSizeAudit   = int64(2e6)
)

// ConfigInit initializes the configuration. It is separate from OtherInit as some configuration
// may be required prior to calling other Init functions.
func ConfigInit(cnfg config.Config, filepathsToDeleteOnReset []string) {
	config.Init(cnfg, checkLogSize, maxLogSize, checkLogSizeAudit, maxLogSizeAudit,
		filepathsToDeleteOnReset)
}

// OtherInit calls all required Init functions.
func OtherInit(authConfig authJWT.Config, mux *http.ServeMux) {
	authJWT.Init(authConfig, mux)
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
