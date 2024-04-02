// Package config is for configuring the application; CLI parsing, log setup, db setup, etc.
package config

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/paulfdunn/go-helper/databaseh/kvs"
	"github.com/paulfdunn/go-helper/logh"
	"github.com/paulfdunn/go-helper/osh"
	"github.com/paulfdunn/go-helper/osh/runtimeh"
)

type Config struct {
	// Values provided by CLI parameters
	// DataSourcePath is the path to the config data source. Set to:
	// filepath.Join(*persistentDirectory, *cnfg.AppName+".config.db")
	DataSourcePath *string `json:",omitempty"`
	// HTTPSPort - see CLI help for description.
	HTTPSPort *int `json:",omitempty"`
	// LogFilepath - see CLI help for description.
	LogFilepath *string `json:",omitempty"`
	// LogLevel - see CLI help for description.
	LogLevel *int `json:",omitempty"`
	// PersistentDirectory - see CLI help for description.
	PersistentDirectory *string `json:",omitempty"`

	// Other - can be passed into Init.
	// AppName is used to populate the Issuer field of the JWT Claims and will be used
	// as the file name, prefix '.db', for the persistent data source.
	AppName *string `json:",omitempty"`
	// AuditLogName is the name used for the logh audit log; defaults to LogName + ".audit"
	AuditLogName *string `json:",omitempty"`
	// DataSourceIsNew is an output of calling Init and indicates that there was no file
	// at DataSourcePath. This can be used by the calling application to perform other initialization
	// of the data source.
	DataSourceIsNew *bool `json:",omitempty"`
	// LogName is the name of the logh log to use.
	LogName *string `json:",omitempty"`
	// Version is for application version information.
	Version *string `json:",omitempty"`
}

const (
	configFileSuffix = ".config.db"
	configKey        = "config"
)

var (
	// DefaultConfig are the default configuration parameters. These come from flags or get
	// set during Init.
	DefaultConfig Config
	configKVS     kvs.KVS
)

// flags for CLI
var (
	httpsPort   = flag.Int("https-port", 8001, "HTTPS port")
	logFilepath = flag.String("log-filepath", "", "Fully qualified path to log file; default (blank) for STDOUT.")
	logLevel    = flag.Int("log-level", int(logh.Debug), fmt.Sprintf("Logging level; default %d. Zero based index into: %v",
		int(logh.Debug), logh.DefaultLevels))
	persistentDirectory = flag.String("persistent-directory", "", "Fully qualified path to directory for persisted data; default to directory of this executable.")
	reset               = flag.Bool("reset", false, "Reset will remove all persisted data for this instance; "+
		"includes user accounts, settings, log files, etc.")
)

// Init initializes the configuration and logging for the application; calls flag.Parse().
// Applicaitons should not call flag.Parse() as flag.Parse() can only be called once per application.
// checkLogSize/maxLogSize - logh parameters for the application log.
// checkLogSizeAudit/maxLogSizeAudit - logh parameters for the audit log.
// filepathsToDeleteOnReset - fully qualified file paths for any files that need deleted on
//
//	application reset via CLI parameter. Uses Glob patterns.
//
// The only required config inputs are: AppName (used to populate the Issuer field of the JWT
// Claims) and LogName.
//
// Init creates the DefaultConfig from the initConfig by adding values from parsing CLI parameters,
// and filling in defaults for Config members without values. The caller can then call Get()
// to merge any saved configuration data into the default data. That configuration can be
// modified at runtime, and saved using Set(), or deleted using Delete()
func Init(initConfig Config, checkLogSize int, maxLogSize int64,
	checkLogSizeAudit int, maxLogSizeAudit int64, filepathsToDeleteOnReset []string) {

	var err error
	flag.Parse()

	// logging setup
	err = logh.New(*initConfig.LogName, *logFilepath, logh.DefaultLevels, logh.LoghLevel(*logLevel),
		logh.DefaultFlags, checkLogSize, maxLogSize)
	if err != nil {
		log.Fatalf("fatal: %s error creating log, error: %v", runtimeh.SourceInfo(), err)
	}
	var auditLogFilepath string
	if logFilepath != nil && *logFilepath != "" {
		auditLogFilepath = *logFilepath + ".audit"
	}
	if initConfig.AuditLogName == nil || *initConfig.AuditLogName == "" {
		aln := *initConfig.LogName + ".audit"
		initConfig.AuditLogName = &aln
	}
	err = logh.New(*initConfig.AuditLogName, auditLogFilepath, logh.DefaultLevels, logh.Audit,
		logh.DefaultFlags, checkLogSizeAudit, maxLogSizeAudit)
	if err != nil {
		log.Fatalf("fatal: %s error creating audit log, error: %v", runtimeh.SourceInfo(), err)
	}

	if initConfig.AppName == nil || initConfig.LogName == nil {
		log.Fatalf("fatal: initConfig.AppName and initConfig.LogName are required to be non-nil.")
	}

	if *persistentDirectory == "" {
		// default to the executable path.
		exe, err := os.Executable()
		if err != nil {
			log.Fatalf("fatal: %s fatal: Could not find executable path.", runtimeh.SourceInfo())
		}
		ap := filepath.Dir(exe)
		persistentDirectory = &ap
	}

	if *persistentDirectory != "" {
		if err := os.MkdirAll(*persistentDirectory, 0755); err != nil {
			log.Fatalf("fatal: %s MkdirAll error: %v", runtimeh.SourceInfo(), err)
		}
	}

	dataSourcePath := filepath.Join(*persistentDirectory, *initConfig.AppName+configFileSuffix)

	// reset if requested - do PRIOR to logging setup as logs are deleted.
	if err = resetIfRequested(*reset, dataSourcePath, filepathsToDeleteOnReset); err != nil {
		log.Fatalf("fatal: %s resetIfRequested error: %v", runtimeh.SourceInfo(), err)
	}

	dataSourceIsNew := false
	if _, err := os.Stat(dataSourcePath); os.IsNotExist(err) {
		logh.Map[*initConfig.LogName].Printf(logh.Info, "dataSourceIsNew = true")
		dataSourceIsNew = true
	}

	logh.Map[*initConfig.LogName].Printf(logh.Info, "%s is starting....", *initConfig.LogName)
	logh.Map[*initConfig.AuditLogName].Printf(logh.Audit, "%s is starting....", *initConfig.LogName)
	logh.Map[*initConfig.LogName].Printf(logh.Info, "logFilepath:%s", *logFilepath)
	logh.Map[*initConfig.LogName].Printf(logh.Info, "auditLogFilepath:%s", auditLogFilepath)

	initializeKVInstance(dataSourcePath)

	DefaultConfig = initConfig
	// CLI
	DefaultConfig.HTTPSPort = httpsPort
	DefaultConfig.LogFilepath = logFilepath
	DefaultConfig.LogLevel = logLevel
	DefaultConfig.PersistentDirectory = persistentDirectory
	DefaultConfig.DataSourcePath = &dataSourcePath
	// Other
	DefaultConfig.AppName = initConfig.AppName
	DefaultConfig.AuditLogName = initConfig.AuditLogName
	DefaultConfig.DataSourceIsNew = &dataSourceIsNew
	DefaultConfig.LogName = initConfig.LogName
	DefaultConfig.Version = initConfig.Version
}

// Set persists the provided Configuration. This can be called dynamically to store application state.
// Any stored state will override default configuration.
func (cnfg *Config) Set() error {
	return runtimeh.SourceInfoError("", configKVS.Serialize(configKey, cnfg))
}

func (cnfg Config) String() string {
	b, _ := json.Marshal(cnfg)
	return string(b)
}

// Delete will remove the stored configuration by deleting the KVS store.
func Delete() error {
	return runtimeh.SourceInfoError("", configKVS.DeleteStore())
}

// Get returns the current configuration. The current configuration is based on default/CLI values,
// but those may be overriden by saved values.
func Get() (Config, error) {
	mergedConfig := DefaultConfig
	err := configKVS.Deserialize(configKey, &mergedConfig)
	return mergedConfig, runtimeh.SourceInfoError("", err)
}

// resetIfRequested - delete all configuration and log data if reset == true.
// filepathsToDeleteOnReset is a slice of glob patterns; all specified files will be deleted.
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
	if configKVS, err = kvs.New(dataSourcePath, configKey); err != nil {
		log.Fatalf("fatal: %s could not create New kvs, error: %v", runtimeh.SourceInfo(), err)
	}
}
