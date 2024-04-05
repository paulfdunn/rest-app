// example-telemetry is an example of using github.com/paulfdunn/rest-app (a framework for a GO
// (GOLANG) based ReST APIs) to create a service that: accepts commands to run, runs those commands
// and collects STDOUT/STDERR, then allows downloading the resulting data plus additional files in a
// ZIP file. This service does not include authentication natively, but relies on a
// example-auth-as-service to provide clients with authentication services. This service is then
// called with a token provisioned from example-auth-as-service, and this service validates the
// token for paths requiring authentication.
//
// See test-example-telemetry.sh for a full working example that uses the API to send an "ls -al"
// command and download the ZIP file.
//
// WARNING - There is no locking on Task objects. Once runner is running, no object updates should
// occur other than those that occur within runner. I.E. don't update a Task in the foreground while
// updates are happening in the background (go routine), or the foreground updates will be lost.
//
// TODO: REGEX filter to reject/limit values in command/shell
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
	"github.com/paulfdunn/go-helper/archiveh/ziph"
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
	Cancel *bool `json:",omitempty"`
	// Command is a slice of strings of commands that are executed WITHOUT a shell.
	Command []string `json:",omitempty"`
	// Expiration - date with format dateFormat; time is UTC, 24 hour notation. Default is
	// defaultExpirationDuration from POST. Upon Expiration a task is Canceled if still running,
	// and all files are deleted.
	Expiration *string `json:",omitempty"`
	// File is a list of files and directories to be collected in returned ZIP file. Globs and wildcards
	// are not supported.
	File []string `json:",omitempty"`
	// ProcessError, ProcessCommand, ProcessShell, ProcessZip are status information provided as the task runs.
	ProcessCommand []string `json:",omitempty"`
	ProcessError   []error  `json:",omitempty"`
	ProcessShell   []string `json:",omitempty"`
	ProcessZip     []string `json:",omitempty"`
	// Shell is a slice of strings of commands that are executed in a shell.
	Shell []string `json:",omitempty"`
	// Status is a value of type TaskStatus; output only, do not provide with PUT/POST.
	Status *TaskStatus `json:",omitempty"`
	// StatusString is Status.String(). This value should only be used to make output human readable, but
	// should never be parsed for data processing; use the numeric value as string output is subject to change.
	// Internally this value is never stored; it is only added to ReST API output as data is returned.
	StatusString string `json:",omitempty"`
	// UUID is a UUID that is returned with a task POST
	UUID *uuid.UUID `json:",omitempty"`
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

// runningTask are used to keep context for running Tasks.
type runningTask struct {
	cancelFunc context.CancelFunc
	ctx        context.Context
	task       *Task
}

// runningTaskMap is a map, with Task.Key(), of runningTasks
type runningTaskMap map[string]runningTask

const (
	dateFormat = "2006-01-02 15:04:05" //UTC, 24 hour notation

	// relative file paths will be joined with appPath to create the path to the file.
	relativeCertFilePath  = "/key/rest-app.crt"
	relativeKeyFilePath   = "/key/rest-app.key"
	relativePublicKeyPath = "../example-auth-as-service/key/jwt.rsa.public"

	stderrFileSuffix = ".stderr.txt"
	stdoutFileSuffix = ".stdout.txt"
	zipFileSuffix    = ".zip"

	// taskKey values are used for testing and in Task.sliceLength
	taskKeyCancel         = "Cancel"
	taskKeyCommand        = "Command"
	taskKeyFile           = "File"
	taskKeyProcessError   = "ProcessError"
	taskKeyExpiration     = "Expiration"
	taskKeyProcessCommand = "ProcessCommand"
	taskKeyProcessShell   = "ProcessShell"
	taskKeyProcessZip     = "ProcessZip"
	taskKeyShell          = "Shell"
	taskKeyStatus         = "Status"
	taskKeyUUID           = "UUID"

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
	taskCancel    chan string
	taskCompleted chan string
	taskRun       chan string
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
	if tsk.UUID.String() == inputTask.UUID.String() &&
		diff <= allowedExpirationDifference &&
		((tsk.Cancel == nil && inputTask.Cancel == nil) || (*tsk.Cancel == *inputTask.Cancel)) &&
		((tsk.Status == nil && inputTask.Status == nil) || (*tsk.Status == *inputTask.Status)) &&
		tsk.sliceLength(taskKeyCommand) == inputTask.sliceLength(taskKeyCommand) &&
		tsk.sliceLength(taskKeyFile) == inputTask.sliceLength(taskKeyFile) &&
		tsk.sliceLength(taskKeyProcessError) == inputTask.sliceLength(taskKeyProcessError) &&
		tsk.sliceLength(taskKeyProcessCommand) == inputTask.sliceLength(taskKeyProcessCommand) &&
		tsk.sliceLength(taskKeyProcessShell) == inputTask.sliceLength(taskKeyProcessShell) &&
		tsk.sliceLength(taskKeyProcessZip) == inputTask.sliceLength(taskKeyProcessZip) &&
		tsk.sliceLength(taskKeyShell) == inputTask.sliceLength(taskKeyShell) {
		if tsk.sliceLength(taskKeyCommand) > 0 {
			for i := 0; i < tsk.sliceLength(taskKeyCommand); i++ {
				if tsk.Command[i] != inputTask.Command[i] {
					return false
				}
			}
		}
		if tsk.sliceLength(taskKeyProcessError) > 0 {
			for i := 0; i < tsk.sliceLength(taskKeyProcessError); i++ {
				if tsk.ProcessError[i] != inputTask.ProcessError[i] {
					return false
				}
			}
		}
		if tsk.sliceLength(taskKeyProcessCommand) > 0 {
			for i := 0; i < tsk.sliceLength(taskKeyProcessCommand); i++ {
				if tsk.ProcessCommand[i] != inputTask.ProcessCommand[i] {
					return false
				}
			}
		}
		if tsk.sliceLength(taskKeyProcessShell) > 0 {
			for i := 0; i < tsk.sliceLength(taskKeyProcessShell); i++ {
				if tsk.ProcessShell[i] != inputTask.ProcessShell[i] {
					return false
				}
			}
		}
		if tsk.sliceLength(taskKeyProcessZip) > 0 {
			for i := 0; i < tsk.sliceLength(taskKeyProcessZip); i++ {
				if tsk.ProcessZip[i] != inputTask.ProcessZip[i] {
					return false
				}
			}
		}
		if tsk.sliceLength(taskKeyShell) > 0 {
			for i := 0; i < tsk.sliceLength(taskKeyShell); i++ {
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

// ZipFilePath creates the path to the created zip file.
func (tsk Task) ZipFilePath() string {
	return filepath.Join(tsk.Dir(), tsk.UUID.String()+zipFileSuffix)
}

// SliceLength is used to get the length of one of the slice fields. This main utility of this function
// is for Equal(ality) testing.
func (tsk *Task) sliceLength(field string) int {
	switch field {
	case taskKeyCommand:
		if tsk.Command == nil {
			return -1
		}
		return len(tsk.Command)
	case taskKeyFile:
		if tsk.File == nil {
			return -1
		}
		return len(tsk.Command)
	case taskKeyProcessError:
		if tsk.ProcessError == nil {
			return -1
		}
		return len(tsk.ProcessError)
	case taskKeyProcessCommand:
		if tsk.ProcessCommand == nil {
			return -1
		}
		return len(tsk.ProcessCommand)
	case taskKeyProcessShell:
		if tsk.ProcessShell == nil {
			return -1
		}
		return len(tsk.ProcessShell)
	case taskKeyProcessZip:
		if tsk.ProcessZip == nil {
			return -1
		}
		return len(tsk.ProcessZip)
	case taskKeyShell:
		if tsk.Shell == nil {
			return -1
		}
		return len(tsk.Shell)
	}
	log.Fatalf("Invalid field provided to SliceLength, field: %s", field)
	return -1
}

// updateTaskStatus will Task.Status and serialize the Task to the KVS.
func (tsk *Task) updateTaskStatus(status TaskStatus) error {
	stts := status
	tsk.Status = &stts
	err := telemetryKVS.Serialize(tsk.Key(), &tsk)
	if err != nil {
		lpf(logh.Error, "Serialize task: %+v", err)
		return err
	}
	return nil
}

// String returns the string corresponding to a TaskStatus.
// These are stored in a database and CANNOT BE REORDERED! Add new values to the end of the list.
func (ts TaskStatus) String() string {
	return [...]string{"Accepted", "Canceled", "Canceling", "Completed", "Expired", "Running"}[ts]
}

// runner does the work for a runningTask; it should be called in a Go routine from ScheduleTasks.
// The work to do: execute all commands in task.Command, and task.Shell, and create the zip
// file. Status is updates as it runs.
// WARNING - There is no locking on Task objects. Once runner is running, no object updates
// should occur other than those that occur within runner. I.E. don't update a Task in the
// foreground while updates are happening in the background (go routine), or the foreground
// updates will be lost.
func (rt runningTask) runner() {
	var filepathsShell []string
	filepathsCmd := rt.runnerExec(true)
	// Task might have been canceled.
	if *rt.task.Status == Running {
		filepathsShell = rt.runnerExec(false)
	}

	// Task.Dir may be relative, make it absolute for trimming.
	trim, err := filepath.Abs(rt.task.Dir())
	if err != nil {
		lpf(logh.Error, "filepath.Abs: %+v", err)
	}
	zipFiles := make([]string, 0, len(filepathsCmd)+len(filepathsShell)+len(rt.task.File))
	zipFiles = append(zipFiles, filepathsCmd...)
	zipFiles = append(zipFiles, filepathsShell...)
	zipFiles = append(zipFiles, rt.task.File...)
	_, processedPaths, errs := ziph.AsyncZip(rt.task.ZipFilePath(), zipFiles, []string{trim})
	for {
		// Task might have been canceled.
		if *rt.task.Status != Running {
			break
		}
		noMessage := false
		select {
		case pp, ok := <-processedPaths:
			if ok {
				lpf(logh.Info, "AsyncZip processed path: %s\n", pp)
				rt.task.ProcessZip = append(rt.task.ProcessZip, pp)
			} else {
				processedPaths = nil
			}
		case err, ok := <-errs:
			if ok {
				lpf(logh.Info, "AsyncZip error: %v\n", err)
				rt.task.ProcessError = append(rt.task.ProcessError, err)
			} else {
				errs = nil
			}
		default:
			noMessage = true
		}

		if noMessage {
			if processedPaths == nil && errs == nil {
				lpf(logh.Info, "AsyncZip is done, filepath: %s", rt.task.ZipFilePath())
				break
			}

			// Save status information.
			if err := telemetryKVS.Serialize(rt.task.Key(), &rt.task); err != nil {
				lpf(logh.Error, "telemetryKVS.Serialize error: %+v", err)
			}

			time.Sleep(time.Second)
		}
	}

	// The task may have been canceled or otherwise have had the status updated.
	// Only change to Completed if the current status is Running.
	if *rt.task.Status == Running {
		cmplt := Completed
		rt.task.Status = &cmplt
	}
	if err := telemetryKVS.Serialize(rt.task.Key(), &rt.task); err != nil {
		lpf(logh.Error, "telemetryKVS.Serialize error: %+v", err)
	}

	lpf(logh.Info, "task %s completed", rt.task.UUID.String())
	taskCompleted <- rt.task.Key()
}

// runnerExec runs the runningTask.task.Command and runningTask.task.Shell commands
func (rt runningTask) runnerExec(command bool) []string {
	var execList []string
	if command {
		execList = rt.task.Command
	} else {
		execList = rt.task.Shell
	}
	// Each command generates 2 files; stdout and stderr
	filepaths := make([]string, 0, len(execList)*2)
	if len(execList) > 0 {
		for _, cmdAndArgs := range execList {
			select {
			case <-rt.ctx.Done():
				cncl := Canceled
				rt.task.Status = &cncl
				if err := telemetryKVS.Serialize(rt.task.Key(), &rt.task); err != nil {
					lpf(logh.Error, "telemetryKVS.Serialize error: %+v", err)
				}
				// return rt.ctx.Err()
				return filepaths
			default:
			}

			cmdSplit := strings.Fields(cmdAndArgs)
			var args []string
			if len(cmdSplit) > 1 {
				args = cmdSplit[1:]
			}

			cmdToFileName := filenameFromCommand(cmdAndArgs)
			stderr, err := os.Create(filepath.Join(rt.task.Dir(), cmdToFileName+stderrFileSuffix))
			if err != nil {
				lpf(logh.Error, "os.Create: %+v", err)
				rt.task.ProcessError = append(rt.task.ProcessError, err)
			}
			defer stderr.Close()
			stdout, err := os.Create(filepath.Join(rt.task.Dir(), cmdToFileName+stdoutFileSuffix))
			if err != nil {
				lpf(logh.Error, "os.Create: %+v", err)
				rt.task.ProcessError = append(rt.task.ProcessError, err)
			}
			defer stdout.Close()
			filepaths = append(filepaths, stderr.Name(), stdout.Name())

			ea := exech.ExecArgs{Args: args, Command: cmdSplit[0], Context: rt.ctx, Stderr: stderr, Stdout: stdout}
			var rc int
			if command {
				rc, err = exech.ExecCommandContext(&ea)
				rt.task.ProcessCommand = append(rt.task.ProcessCommand, cmdAndArgs)
			} else {
				rc, err = exech.ExecShellContext(&ea)
				rt.task.ProcessShell = append(rt.task.ProcessShell, cmdAndArgs)
			}
			if rc != 0 {
				lpf(logh.Error, "non-zero return code %d for command: %t, cmdAndArgs: %s", rc, command, cmdAndArgs)
			}
			if err != nil {
				rt.task.ProcessError = append(rt.task.ProcessError, err)
			}

			// Save status information.
			if err := telemetryKVS.Serialize(rt.task.Key(), &rt.task); err != nil {
				lpf(logh.Error, "telemetryKVS.Serialize error: %+v", err)
			}
		}
	}
	return filepaths
}

func (rtm runningTaskMap) cancelExpiredTasks() {
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

func (rtm runningTaskMap) scheduleTasks() {
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
			if err = dtask.updateTaskStatus(Running); err != nil {
				lpf(logh.Error, "updateTaskStatus: %+v", err)
			}
			rt := runningTask{cancelFunc: cancelFunc, ctx: ctx, task: &dtask}
			rtm[key] = rt
			go rt.runner()
		default:
		}
	}
}

// deleteExpiredTasks should be called at startup, prior to starting any task management,
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
			if err = dtask.updateTaskStatus(Canceled); err != nil {
				lpf(logh.Error, "updateTaskStatus: %+v", err)
			}
		}
	}
}

func filenameFromCommand(cmd string) string {
	// Replace characters in the command that are not valid for a file name.
	re := regexp.MustCompile(`[` + "`" + ` ~!@#$%^&*()+=\{\[\}\]\|\?\\/><,\.';:"]+`)
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
	taskCompleted = make(chan string)
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

// taskRunner accepts new tasks on the taskRun channel. Callers will be blocked until the task
// is accepted.
func taskRunner() {
	runningTasks := make(runningTaskMap)
	for {
		select {
		case key := <-taskCancel:
			if _, ok := runningTasks[key]; ok {
				// Status is set when cancel is recognized
				runningTasks[key].cancelFunc()
				delete(runningTasks, key)
				lpf(logh.Info, "task %s canceled", key)
			} else {
				dtask := Task{}
				err := telemetryKVS.Deserialize(key, &dtask)
				if err != nil {
					log.Fatalf("Deserialize task: %+v", err)
				}
				if err = dtask.updateTaskStatus(Canceled); err != nil {
					lpf(logh.Error, "updateTaskStatus: %+v", err)
				}
				lpf(logh.Warning, "taskRunner received non-running task %s to cancel", key)
			}
			// Allow cancelation requests to short circuit the loop as they may be done programatically
			// and it would be best to not block callers.
			continue
		case key := <-taskCompleted:
			delete(runningTasks, key)
			lpf(logh.Info, "task %s removed from runningTasks", key)
			// Allow another task to start quickly it there is one waiting.
			continue
		default:
		}

		runningTasks.cancelExpiredTasks()
		runningTasks.scheduleTasks()

		time.Sleep(taskRunnerCycleTime)
	}
}
