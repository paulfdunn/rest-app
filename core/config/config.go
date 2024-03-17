// Package config is for configuring the application; CLI parsing, log setup, db setup, etc.
package config

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/paulfdunn/go-helper/databaseh/kvs"
	"github.com/paulfdunn/go-helper/logh"
	"github.com/paulfdunn/go-helper/osh"
	"github.com/paulfdunn/go-helper/osh/runtimeh"
)

type Config struct {
	// CLI parameters
	HTTPSPort   *int    `json:",omitempty"`
	LogFilepath *string `json:",omitempty"`
	LogLevel    *int    `json:",omitempty"`

	// Other
	AppName                   *string        `json:",omitempty"`
	AppPath                   *string        `json:",omitempty"`
	AuditLogName              *string        `json:",omitempty"`
	DataSourcePath            *string        `json:",omitempty"`
	JWTKeyPath                *string        `json:",omitempty"`
	JWTAuthRemoveInterval     *time.Duration `json:",omitempty"`
	JWTAuthExpirationInterval *time.Duration `json:",omitempty"`
	LogName                   *string        `json:",omitempty"`
	NewDataSource             *bool          `json:",omitempty"`
	PasswordValidation        []string       `json:",omitempty"`
	PersistentDirectory       *string        `json:",omitempty"`
	Version                   *string        `json:",omitempty"`
}

const (
	configKey = "config"
)

var (
	// DefaultConfig are the default configuration parameters. These come from flags and/or the application
	// during Init.
	DefaultConfig Config
	kvi           kvs.KVS
)

// flags for CLI
var (
	httpsPort   = flag.Int("https-port", 8080, "HTTPS port")
	logFilepath = flag.String("log-filepath", "", "Fully qualified path to log file; default (blank) for STDOUT.")
	logLevel    = flag.Int("log-level", int(logh.Debug), fmt.Sprintf("Logging level; default %d. Zero based index into: %v",
		int(logh.Debug), logh.DefaultLevels))
	persistentDirectory = flag.String("persistent-directory", "", "Fully qualified path to directory for persisted data; default to directory of this executable.")
	reset               = flag.Bool("reset", false, "Reset will remove all persisted data for this instance; "+
		"includes user accounts, settings, log files, etc.")
)

// Init initializes the configuration and logging for the application.
// checkLogSize/maxLogSize - logh parameters for the application log.
// checkLogSizeAudit/maxLogSizeAudit - logh parameters for the audit log.
// filepathsToDeleteOnReset - fully qualified file paths for any files that needs deleted on
// application reset. Uses Glob patterns.
func Init(cnfg Config, checkLogSize int, maxLogSize int64,
	checkLogSizeAudit int, maxLogSizeAudit int64, filepathsToDeleteOnReset []string) {

	var err error
	flag.Parse()

	if cnfg.AppName == nil || cnfg.AppPath == nil || cnfg.LogName == nil {
		log.Fatalf("fatal: cnfg.AppName, cnf.AppPath, and cnfg.LogName are required to be non-nil.")
	}

	if *persistentDirectory == "" {
		persistentDirectory = cnfg.AppPath
	}

	if *persistentDirectory != "" {
		if err := os.MkdirAll(*persistentDirectory, 0755); err != nil {
			log.Fatalf("fatal: %s MkdirAll error: %v", runtimeh.SourceInfo(), err)
		}
	}

	dataSourcePath := filepath.Join(*persistentDirectory, *cnfg.AppName+".db")
	newDataSource := false
	if _, err := os.Stat(dataSourcePath); os.IsNotExist(err) {
		newDataSource = true
	}

	// reset if requested - do PRIOR to logging setup as logs are deleted.
	if err = resetIfRequested(*reset, dataSourcePath, filepathsToDeleteOnReset); err != nil {
		log.Fatalf("fatal: %s resetIfRequested error: %v", runtimeh.SourceInfo(), err)
	}

	// logging setup
	err = logh.New(*cnfg.LogName, *logFilepath, logh.DefaultLevels, logh.LoghLevel(*logLevel),
		logh.DefaultFlags, checkLogSize, maxLogSize)
	if err != nil {
		log.Fatalf("fatal: %s error creating log, error: %v", runtimeh.SourceInfo(), err)
	}
	var auditLogName, auditLogFilepath string
	if *logFilepath != "" {
		auditLogName = *cnfg.AuditLogName + ".audit"
		auditLogFilepath = *logFilepath + ".audit"
	}
	err = logh.New(auditLogName, auditLogFilepath, logh.DefaultLevels, logh.Audit,
		logh.DefaultFlags, checkLogSizeAudit, maxLogSizeAudit)
	if err != nil {
		log.Fatalf("fatal: %s error creating audit log, error: %v", runtimeh.SourceInfo(), err)
	}
	logh.Map[*cnfg.LogName].Printf(logh.Info, "%s is starting....", *cnfg.LogName)
	logh.Map[auditLogName].Printf(logh.Audit, "%s is starting....", *cnfg.LogName)
	logh.Map[*cnfg.LogName].Printf(logh.Info, "logFilepath:%s", *logFilepath)
	logh.Map[*cnfg.LogName].Printf(logh.Info, "auditLogFilepath:%s", auditLogFilepath)

	initializeKVInstance(dataSourcePath)

	DefaultConfig = cnfg
	// CLI
	DefaultConfig.HTTPSPort = httpsPort
	DefaultConfig.LogFilepath = logFilepath
	DefaultConfig.LogLevel = logLevel
	DefaultConfig.PersistentDirectory = persistentDirectory
	// Other
	DefaultConfig.AppName = cnfg.AppName
	DefaultConfig.AuditLogName = &auditLogName
	DefaultConfig.DataSourcePath = &dataSourcePath
	DefaultConfig.LogName = cnfg.LogName
	DefaultConfig.NewDataSource = &newDataSource
}

// Set persists the provided Configuration.
func (cnfg *Config) Set() error {
	return runtimeh.SourceInfoError("", kvi.Serialize(configKey, cnfg))
}

func (cnfg Config) String() string {
	b, _ := json.Marshal(cnfg)
	return string(b)
}

// Delete will remove the stored configuration by deleting the KVS store.
func Delete() error {
	return runtimeh.SourceInfoError("", kvi.DeleteStore())
}

// Get returns the current configuration. The current configuration is based on default/CLI values,
// but those may be overriden by saved values.
func Get() (Config, error) {
	mergedConfig := DefaultConfig
	err := kvi.Deserialize(configKey, &mergedConfig)
	return mergedConfig, runtimeh.SourceInfoError("", err)
}

// resetIfRequested - delete all application data if reset == true.
func resetIfRequested(reset bool, dataSourcePath string, filepathsToDeleteOnReset []string) error {
	var errOut error
	if reset {
		// If the dataSourcePath is a file, delete it.
		if _, err := os.Stat(dataSourcePath); err == nil {
			err := os.Remove(dataSourcePath)
			if err != nil {
				errOut = fmt.Errorf("deleting file: %s, error: %v, prior errors: %v", dataSourcePath, err, errOut)
			}
		}

		for _, v := range filepathsToDeleteOnReset {
			err := osh.RemoveAllFiles(v)
			if err != nil {
				errOut = fmt.Errorf("deleting file: %s, error: %v, prior errors: %v", v, err, errOut)
			}
		}

		if *logFilepath != "" {
			err := osh.RemoveAllFiles(*logFilepath + "*")
			if err != nil {
				errOut = fmt.Errorf("deleting logs: %v, prior errors: %v", err, errOut)
			}
		}
	}
	return runtimeh.SourceInfoError("", errOut)
}

// initializeKVInstance - Initialize the KVS
func initializeKVInstance(dataSourcePath string) {
	var err error
	// The KVS table name and key will both use configKey.
	if kvi, err = kvs.New(dataSourcePath, configKey); err != nil {
		log.Fatalf("fatal: %s could not create New kvs, error: %v", runtimeh.SourceInfo(), err)
	}
}
