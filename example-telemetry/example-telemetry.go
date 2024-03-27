// example-telemetry is an example of using github.com/paulfdunn/rest-app (a framework for a
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

	"github.com/google/uuid"

	"github.com/paulfdunn/authJWT"
	"github.com/paulfdunn/go-helper/databaseh/kvs"
	"github.com/paulfdunn/go-helper/logh"
	"github.com/paulfdunn/go-helper/osh/runtimeh"
	"github.com/paulfdunn/rest-app/core"
	"github.com/paulfdunn/rest-app/core/config"
)

type Task struct {
	// Cancel -  Not valid on POST. If set to true on PUT will cancel the task; providing any other
	// field besides UUID with a PUT is not valid.
	Cancel *bool
	// Command is a slice of strings of commands that are executed WITHOUT a shell.
	Command []string
	// Expiration - date with format dateFormat; time is UTC, 24 hour notation. Default is
	// defaultExpirationDuration from POST. Upon Expiration a task is Canceled if still running,
	// and all files are deleted.
	Expiration *string
	// Shell is a slice of strings of commands that are executed in a shell.
	Shell []string
	// Status is a value of type TaskStatus; output only, do not provide with PUT/POST.
	Status *TaskStatus
	// UUID is a UUID that is returned with a task POST
	UUID *uuid.UUID
}

// TaskStatus are the valid states of a Task.Status.
// These are stored in a database and CANNOT BE REORDERED! Add new values to the end of the list.
type TaskStatus int

const (
	Accepted TaskStatus = iota
	Canceled
	Canceling
	Completed
	Expired
	Running
)

const (
	dateFormat                = "2006-01-02 15:04:05" //UTC, 24 hour notation
	defaultExpirationDuration = 24 * time.Hour

	pathTask   = "/task/"
	pathStatus = "/status/"

	// relative file paths will be joined with appPath to create the path to the file.
	relativeCertFilePath  = "/key/rest-app.crt"
	relativeKeyFilePath   = "/key/rest-app.key"
	relativePublicKeyPath = "../example-standalone/key/jwt.rsa.public"

	telemetryFileSuffix = ".db"
	telemetryTable      = "telemetry"
)

var (
	appName = "example-telemetry"

	// API timeouts
	apiReadTimeout  = 10 * time.Second
	apiWriteTimeout = 10 * time.Second

	// Any files in this list will be deleted on application reset using the CLI parameter
	// See core/config.Init
	filepathsToDeleteOnReset = []string{}

	// logh function pointers make logging calls more compact, but are optional.
	lp  func(level logh.LoghLevel, v ...interface{})
	lpf func(level logh.LoghLevel, format string, v ...interface{})

	runtimeConfig config.Config

	// The KVS stores all task data
	telemetryKVS kvs.KVS
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
	if runtimeConfig, err = config.Get(); err != nil {
		log.Fatalf("fatal: %s getting running config, error:%v", runtimeh.SourceInfo(), err)
	}
	lpf(logh.Info, "Config: %s", runtimeConfig)

	publicKeyPath := filepath.Join(appPath, relativePublicKeyPath)
	ac := authJWT.Config{
		AppName:          *runtimeConfig.AppName,
		JWTPublicKeyPath: publicKeyPath,
		LogName:          *runtimeConfig.LogName,
	}
	mux := http.NewServeMux()
	core.OtherInit(&ac, nil, nil)

	initializeKVS(filepath.Dir(*runtimeConfig.DataSourcePath), *runtimeConfig.AppName+telemetryFileSuffix)

	// Registering with the trailing slash means the naked path is redirected to this path.
	path := "/"
	mux.HandleFunc(path, authJWT.HandlerFuncAuthJWTWrapper(handlerRoot))
	lpf(logh.Info, "Registered handler: %s\n", path)
	mux.HandleFunc(pathStatus, authJWT.HandlerFuncAuthJWTWrapper(handlerStatus))
	lpf(logh.Info, "Registered handler: %s\n", pathStatus)
	mux.HandleFunc(pathTask, authJWT.HandlerFuncAuthJWTWrapper(handlerTask))
	lpf(logh.Info, "Registered handler: %s\n", pathTask)

	cfp := filepath.Join(appPath, relativeCertFilePath)
	kfp := filepath.Join(appPath, relativeKeyFilePath)
	// blocking call
	core.ListenAndServeTLS(appName, mux, fmt.Sprintf(":%d", *runtimeConfig.HTTPSPort),
		apiReadTimeout, apiWriteTimeout, cfp, kfp)
}

// Equal compares two Task objects and determines equality of values. UUID is the key for the
// kvs thus this test doesn't check for UUID == nil. Similarly, expiration will never be nil
// because a default is provided.
// allowedExpirationDifference can be used with a non-zero value to check for equality when
// a task was created with default Expiration, thus the caller may not know the exact time
// the Expiration was set to.
func (tsk *Task) Equal(inputTask *Task, allowedExpirationDifference time.Duration) bool {
	tskExp, err := time.Parse(dateFormat, *tsk.Expiration)
	if err != nil {
		lpf(logh.Error, "parsing tsk.Expiration: %+v", err)
		return false
	}
	inputExp, err := time.Parse(dateFormat, *inputTask.Expiration)
	if err != nil {
		lpf(logh.Error, "parsing inputTask.Expiration: %+v", err)
		return false
	}
	diff := tskExp.Sub(inputExp).Abs()
	if tsk.UUID.String() == inputTask.UUID.String() &&
		diff <= allowedExpirationDifference &&
		tsk.SliceLength("Command") == inputTask.SliceLength("Command") &&
		tsk.SliceLength("Shell") == inputTask.SliceLength("Shell") {
		if tsk.SliceLength("Command") > 0 {
			for i := 0; i < tsk.SliceLength("Command"); i++ {
				if tsk.Command[i] != inputTask.Command[i] {
					return false
				}
			}
		}
		if tsk.SliceLength("Shell") > 0 {
			for i := 0; i < tsk.SliceLength("Shell"); i++ {
				if tsk.Shell[i] != inputTask.Shell[i] {
					return false
				}
			}
		}
		return true
	}
	return false
}

// Dir returns the filepath to data directory for the task.
func (tsk *Task) Dir() string {
	return filepath.Join(*runtimeConfig.PersistentDirectory, tsk.UUID.String())
}

// SliceLength is used to get the length of one of the slice fields. This main utility of this function
// is for Equal(ality) testing.
func (tsk *Task) SliceLength(field string) int {
	switch field {
	case "Command":
		if tsk.Command == nil {
			return -1
		}
		return len(tsk.Command)
	case "Shell":
		if tsk.Shell == nil {
			return -1
		}
		return len(tsk.Shell)
	}
	log.Fatalf("Invalid field provided to SliceLength, field: %s", field)
	return -1
}

// String returns the string corresponding to a TaskStatus.
// These are stored in a database and CANNOT BE REORDERED! Add new values to the end of the list.
func (ts TaskStatus) String() string {
	return [...]string{"Accepted", "Canceled", "Canceling", "Completed", "Expired", "Running"}[ts]
}

// initializeKVS initializes the KVS, creating a new KVS if required otherwise attaching to the existing KVS.
func initializeKVS(datasourcePath string, filename string) {
	dataSourcePath := filepath.Join(datasourcePath, filename)
	lpf(logh.Info, "telemetryKVS path: %s", dataSourcePath)
	var err error
	if telemetryKVS, err = kvs.New(dataSourcePath, telemetryTable); err != nil {
		log.Fatalf("fatal: %s fatal: could not create New kvs, error: %v", runtimeh.SourceInfo(), err)
	}
}
