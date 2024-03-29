// example-telemetry is an example of using github.com/paulfdunn/rest-app (a framework for a
// GO (GOLANG) based ReST APIs) to create a service that does not include authentication, but
// relies on a separate auth service to provide clients with authentication service. This service
// is then called with a token provisioned from the separate authentication service, and this
// service validates the token for paths requiring authentication.
//
// TODO: REGEX filter to reject unallowed command/shell
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/paulfdunn/authJWT"
	"github.com/paulfdunn/go-helper/databaseh/kvs"
	"github.com/paulfdunn/go-helper/logh"
	"github.com/paulfdunn/go-helper/osh/exech"
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

// type StatusDetail struct {
// 	Done     *bool
// 	Errors   []error
// 	Executed *int
// }

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

type runningTask struct {
	cancelFunc context.CancelFunc
	ctx        context.Context
	processed  chan<- string
	errors     chan<- error
	task       *Task
}

type runningTaskMap map[string]runningTask

const (
	dateFormat = "2006-01-02 15:04:05" //UTC, 24 hour notation

	// relative file paths will be joined with appPath to create the path to the file.
	relativeCertFilePath  = "/key/rest-app.crt"
	relativeKeyFilePath   = "/key/rest-app.key"
	relativePublicKeyPath = "../example-standalone/key/jwt.rsa.public"

	stderrFileSuffix = ".stderr.txt"
	stdoutFileSuffix = ".stdout.txt"

	taskKeyCommand = "Command"
	taskKeyShell   = "Shell"

	taskDataDirectory   = "taskdata"
	telemetryFileSuffix = ".db"
	telemetryTable      = "telemetry"
)

const (
	pathTask   = "/task/"
	pathStatus = "/status/"

	queryParamUUID = "uuid"
)

const (
	defaultExpirationDuration = 24 * time.Hour
	// postScheduleLimit is the maximum time a POST will be blocked while waiting to accept a new command.
	postScheduleLimit   = 3 * taskRunnerCycleTime
	taskRunnerCycleTime = time.Duration(time.Second)
)

var (
	// maxTasks is the maximum number of tasks that can be running in parallel.
	// This is a variable so it can be increased during testing.
	maxTasks = 5
	// task channels take a Task.Key()
	taskCancel chan string
	taskRun    chan string
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

	path := "/"
	mux.HandleFunc(path, authJWT.HandlerFuncAuthJWTWrapper(handlerRoot))
	lpf(logh.Info, "Registered handler: %s\n", path)
	mux.HandleFunc(pathStatus, authJWT.HandlerFuncAuthJWTWrapper(handlerStatus))
	lpf(logh.Info, "Registered handler: %s\n", pathStatus)
	mux.HandleFunc(pathTask, authJWT.HandlerFuncAuthJWTWrapper(handlerTask))
	lpf(logh.Info, "Registered handler: %s\n", pathTask)

	deleteExpiredTasks()
	initializeTaskInfrastructure()
	startupAddRunningTasks()

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
	if tsk.Key() == inputTask.Key() &&
		diff <= allowedExpirationDifference &&
		tsk.SliceLength(taskKeyCommand) == inputTask.SliceLength(taskKeyCommand) &&
		tsk.SliceLength(taskKeyShell) == inputTask.SliceLength(taskKeyShell) {
		if tsk.SliceLength(taskKeyCommand) > 0 {
			for i := 0; i < tsk.SliceLength(taskKeyCommand); i++ {
				if tsk.Command[i] != inputTask.Command[i] {
					return false
				}
			}
		}
		if tsk.SliceLength(taskKeyShell) > 0 {
			for i := 0; i < tsk.SliceLength(taskKeyShell); i++ {
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
	return filepath.Join(*runtimeConfig.PersistentDirectory, taskDataDirectory, tsk.Key())
}

// Key returns the key used for storing/retrieving a task from telemetryKVS. Having this in a
// function will make changing the key to something else easier. I.E. maybe add the JWT Email so
// to the key so only the owner can operator on the task.
func (tsk *Task) Key() string {
	return tsk.UUID.String()
}

// SliceLength is used to get the length of one of the slice fields. This main utility of this function
// is for Equal(ality) testing.
func (tsk *Task) SliceLength(field string) int {
	switch field {
	case taskKeyCommand:
		if tsk.Command == nil {
			return -1
		}
		return len(tsk.Command)
	case taskKeyShell:
		if tsk.Shell == nil {
			return -1
		}
		return len(tsk.Shell)
	}
	log.Fatalf("Invalid field provided to SliceLength, field: %s", field)
	return -1
}

func (rtm runningTaskMap) CancelExpiredTasks() {
	// Check running tasks for expiration and cancel when expired.
	for key := range rtm {
		dtask := Task{}
		err := telemetryKVS.Deserialize(key, &dtask)
		if err != nil {
			lpf(logh.Error, "Deserialize task: %+v", err)
		}
		exp, err := time.Parse(dateFormat, *dtask.Expiration)
		if err != nil {
			log.Fatalf("time.Parse: %+v", err)
		}
		if dtask.Expiration != nil && time.Now().After(exp) {
			lpf(logh.Info, "running task %s expired and is being canceled", key)
			taskCancel <- key
		}
	}
}

func (rt runningTask) runner() {
	// Size the errors channel large enough for each task plus an error each creating stdout/stderr.
	rt.errors = make(chan error, len(rt.task.Command)+len(rt.task.Shell)+2)
	// Size the processed channel large enough for each task to return its command.
	rt.processed = make(chan string, len(rt.task.Command)+len(rt.task.Shell))

	rt.runnerExec(true)
	rt.runnerExec(false)
	err := updateTaskStatus(rt.task.Key(), Completed)
	if err != nil {
		lpf(logh.Error, "updateTaskStatus: %+v", err)
	}
}

func (rt runningTask) runnerExec(command bool) {
	var execList []string
	if command {
		execList = rt.task.Command
	} else {
		execList = rt.task.Shell
	}
	if execList != nil && len(execList) > 0 {
		for _, cmdAndArgs := range execList {
			select {
			case <-rt.ctx.Done():
				// return rt.ctx.Err()
				return
			default:
			}

			cmdSplit := strings.Fields(cmdAndArgs)
			var args []string
			if len(cmdSplit) > 1 {
				args = cmdSplit[1:]
			}

			cmdToFileName := filenameFromCommand(cmdSplit[0])
			stderr, err := os.Create(filepath.Join(rt.task.Dir(), cmdToFileName+stderrFileSuffix))
			if err != nil {
				lpf(logh.Error, "os.Create: %+v", err)
				rt.errors <- err
			}
			defer stderr.Close()
			stdout, err := os.Create(filepath.Join(rt.task.Dir(), cmdToFileName+stdoutFileSuffix))
			if err != nil {
				lpf(logh.Error, "os.Create: %+v", err)
				rt.errors <- err
			}
			defer stdout.Close()
			// var stdout, stderr bytes.Buffer

			ea := exech.ExecArgs{Args: args, Command: cmdSplit[0], Context: rt.ctx, Stderr: stderr, Stdout: stdout}
			var rc int
			if command {
				rc, err = exech.ExecCommandContext(&ea)
			} else {
				rc, err = exech.ExecShellContext(&ea)
			}
			if rc != 0 {
				lpf(logh.Error, "non-zero return code %d for command: %t, cmdAndArgs: %s", rc, command, cmdAndArgs)
			}
			if err != nil {
				rt.errors <- err
			}
			rt.processed <- cmdSplit[0]

			// TODO: output status data
			lpf(logh.Info, "task %s completed", rt.task.UUID.String())
		}
	}
}

func (rtm runningTaskMap) ScheduleTasks() {
	if len(rtm) < maxTasks {
		select {
		case key := <-taskRun:
			dtask := Task{}
			err := telemetryKVS.Deserialize(key, &dtask)
			if err != nil {
				lpf(logh.Error, "Deserialize task: %+v", err)
				return
			}
			if dtask.UUID == nil || dtask.Status == nil {
				lpf(logh.Error, "Deserialize task has nil UUID: %s", key)
				return
			}
			lpf(logh.Info, "ScheduleTasks accepting task %s", key)
			ctx, cancelFunc := context.WithCancel(context.Background())
			rt := runningTask{cancelFunc: cancelFunc, ctx: ctx, task: &dtask}
			rtm[key] = rt
			err = updateTaskStatus(key, Running)
			if err != nil {
				lpf(logh.Error, "updateTaskStatus: %+v", err)
			}
			go rt.runner()
		default:
		}
	}
}

// String returns the string corresponding to a TaskStatus.
// These are stored in a database and CANNOT BE REORDERED! Add new values to the end of the list.
func (ts TaskStatus) String() string {
	return [...]string{"Accepted", "Canceled", "Canceling", "Completed", "Expired", "Running"}[ts]
}

// deleteExpiredTasks shoudl be called at startup, prior to starting any task management,
// to remove expired tasks from the datastore.
func deleteExpiredTasks() {
	keys, err := telemetryKVS.Keys()
	if err != nil {
		lpf(logh.Error, "Could not read keys from kvs: %+v", err)
	}

	for _, key := range keys {
		dtask := Task{}
		err := telemetryKVS.Deserialize(key, &dtask)
		if err != nil {
			log.Fatalf("Deserialize task: %+v", err)
		}
		// A Task in the KVS should always have non-nil expiration.
		if dtask.Expiration == nil {
			lpf(logh.Error, "Task has nil expiration in KVS: %s", key)
		}
		exp, err := time.Parse(dateFormat, *dtask.Expiration)
		if err != nil {
			log.Fatalf("time.Parse: %+v", err)
		}
		if dtask.Expiration != nil && time.Now().After(exp) {
			if err := os.RemoveAll(dtask.Dir()); err != nil {
				lpf(logh.Error, "delete data directory %s error:%v", dtask.Dir(), err)
			}
			err := updateTaskStatus(key, Canceled)
			if err != nil {
				lpf(logh.Error, "updateTaskStatus: %+v", err)
			}
		}
	}
}

func filenameFromCommand(cmd string) string {
	// Replace characters in the command that are not valid for a file name.
	re := regexp.MustCompile(`[` + "`" + `~!@#$%^&*()+=\{\[\}\]\|\?\\/><,\.';:"]+`)
	return re.ReplaceAllString(cmd, "_")
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

func initializeTaskInfrastructure() {
	taskRun = make(chan string)
	taskCancel = make(chan string)
	go func() {
		taskRunner()
	}()
}

// startupAddRunningTasks is used at startup to re-schedule any tasks that were in progress
// during an application restart.
func startupAddRunningTasks() {
	keys, err := telemetryKVS.Keys()
	if err != nil {
		log.Fatalf("Could not read keys from kvs: %+v", err)
	}

	for _, v := range keys {
		dtask := Task{}
		err := telemetryKVS.Deserialize(v, &dtask)
		if err != nil {
			log.Fatalf("Deserialize task: %+v", err)
		}
		if dtask.Status != nil && slices.Contains([]TaskStatus{Accepted, Running}, *dtask.Status) {
			taskRun <- v
		}
	}
}

// updateTaskStatus will set the Task.Status to Task.Status
func updateTaskStatus(key string, status TaskStatus) error {
	dtask := Task{}
	err := telemetryKVS.Deserialize(key, &dtask)
	if err != nil {
		lpf(logh.Error, "Deserialize task: %+v", err)
		return err
	}
	if dtask.UUID == nil || *dtask.UUID == uuid.Nil {
		lpf(logh.Error, "updateTaskStatus received invalid task %s to cancel", key)
	}
	stts := status
	dtask.Status = &stts
	err = telemetryKVS.Serialize(key, &dtask)
	if err != nil {
		lpf(logh.Error, "Serialize task: %+v", err)
		return err
	}
	return nil
}

// taskRunner accepts new tasks on the taskRun channel. Callers will be blocked until the task
// is accepted.
func taskRunner() {
	runningTasks := make(runningTaskMap)
	for {
		// TODO - this is incorrect. This needs to get the channels, send cancel, wait for channels to close.
		select {
		case key := <-taskCancel:
			if _, ok := runningTasks[key]; ok {
				runningTasks[key].cancelFunc()
				err := updateTaskStatus(key, Canceled)
				if err != nil {
					lpf(logh.Error, "updateTaskStatus: %+v", err)
				}
				delete(runningTasks, key)
				lpf(logh.Info, "task %s canceled", key)
			} else {
				lpf(logh.Error, "taskRunner received invalid task %s to cancel", key)
			}
			// Allow cancelation requests to short circuit the loop as they may be done programatically
			// and it would be best to not block callers.
			continue
		default:
		}

		runningTasks.CancelExpiredTasks()
		runningTasks.ScheduleTasks()

		time.Sleep(taskRunnerCycleTime)
	}
}
