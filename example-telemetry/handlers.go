package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/paulfdunn/authJWT"
	"github.com/paulfdunn/go-helper/logh"
	"github.com/paulfdunn/go-helper/neth/httph"
)

// handlerRoot does nothing - it is just an example of creating a handler and is used by the example
// script so there is an authenticated endpoint to hit.
func handlerRoot(w http.ResponseWriter, r *http.Request) {
	lpf(logh.Debug, "handlerRoot http.request: %v\n", *r)
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
		lpf(logh.Error, "hostname error: %v\n", err)
	}
	w.Header().Set("Content-Type", "text/plain")
	_, err = w.Write([]byte(fmt.Sprintf("hostname: %s, rest-app - from github.com/paulfdunn/rest-app/example-telemetry", hostname)))
	if err != nil {
		lpf(logh.Error, "handlerRoot error: %v\n", err)
	}
}

// handlerStatus
// http.MethodGet - get status for all tasks or tasks specified using queryParamUUID
func handlerStatus(w http.ResponseWriter, r *http.Request) {
	lpf(logh.Debug, "handlerStatus http.request: %v\n", *r)

	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	keys, err := telemetryKVS.Keys()
	if err != nil {
		lpf(logh.Error, "could not get keys from database: %+v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	query := r.URL.Query()
	uuids, filterUUID := query[queryParamUUID]
	var tasks []Task
	if filterUUID {
		tasks = make([]Task, len(uuids))
	} else {
		tasks = make([]Task, len(keys))
	}
	index := 0
	for i, key := range keys {
		dtask := Task{}
		err := telemetryKVS.Deserialize(key, &dtask)
		if err != nil {
			lpf(logh.Error, "could not deserialize task: %+v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if filterUUID && slices.Contains(uuids, key) {
			tasks[index] = dtask
			index++
		} else if !filterUUID {
			tasks[i] = dtask
		}
	}
	b, err := json.Marshal(tasks)
	if err != nil {
		lpf(logh.Error, "json.Marshal error:%v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(b)
}

// handlerTask
// http.MethodDelete - deletes files for Task.UUID; TaskStatus MUST be Canceled, Completed, or Expired.
// Providing any other fields is invalid. Deleting a task is accomplished with a http.MethodPut with
// an Expiration date in the past.
// http.MethodGet - fetch files for a task for Task.UUID; TaskStatus MUST be Canceled, Completed, or Expired.
// Providing any other fields is invalid.
// http.MethodPost - create a new task. It is invalid to supply a UUID or Status in the POST.
// http.MethodPut - with Cancel key 'true' and valid UUID to cancel; providing any other fields or
// value 'false' will error. (Tasks cannot be un-canceled.)
func handlerTask(w http.ResponseWriter, r *http.Request) {
	lpf(logh.Debug, "handlerTask http.request: %v\n", *r)

	if r.Method != http.MethodDelete && r.Method != http.MethodGet &&
		r.Method != http.MethodPost && r.Method != http.MethodPut {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if r.Method == http.MethodDelete {
		taskDelete(w, r)
		return
	}

	if r.Method == http.MethodGet {
		taskGet(w, r)
		return
	}

	if r.Method == http.MethodPost {
		taskPost(w, r)
		return
	}

	if r.Method == http.MethodPut {
		taskPut(w, r)
		return
	}
}

func taskDelete(w http.ResponseWriter, r *http.Request) {
	task := Task{}
	if err := httph.BodyUnmarshal(w, r, &task); err != nil {
		lpf(logh.Error, "taskDelete error:%v", err)
		return
	}
	if task.Cancel != nil || task.Command != nil || task.Expiration != nil || task.Shell != nil || task.Status != nil ||
		task.UUID == nil || *task.UUID == uuid.Nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	dtask := Task{}
	err := telemetryKVS.Deserialize(task.Key(), &dtask)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if dtask.UUID == nil || dtask.Status == nil ||
		(*dtask.Status != Canceled && *dtask.Status != Completed && *dtask.Status != Expired) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if err := os.RemoveAll(task.Dir()); err != nil {
		lpf(logh.Error, "delete data directory %s error:%v", task.Dir(), err)
		w.WriteHeader(http.StatusInternalServerError)
	}

	if aw, ok := w.(*authJWT.AuditWriter); ok {
		aw.Message = fmt.Sprintf("delete issued for task with UUID: %s", *task.UUID)
	}
	w.WriteHeader(http.StatusNoContent)
}

func taskGet(w http.ResponseWriter, r *http.Request) {
	task := Task{}
	if err := httph.BodyUnmarshal(w, r, &task); err != nil {
		lpf(logh.Error, "taskGet error:%v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if task.Cancel != nil || task.Command != nil || task.Expiration != nil || task.Shell != nil || task.Status != nil ||
		task.UUID == nil || *task.UUID == uuid.Nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	dtask := Task{}
	err := telemetryKVS.Deserialize(task.Key(), &dtask)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if dtask.UUID == nil || dtask.Status == nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/x-gzip")
	w.Header().Set("Content-Disposition", "attachment; filename="+filepath.Base(task.ZipFilePath()))
	http.ServeFile(w, r, task.ZipFilePath())
}

func taskPost(w http.ResponseWriter, r *http.Request) {
	task := Task{}
	if err := httph.BodyUnmarshal(w, r, &task); err != nil {
		lpf(logh.Error, "taskPost error:%v", err)
		return
	}
	if task.Cancel != nil || task.Status != nil || (task.UUID != nil && *task.UUID != uuid.Nil) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var expiration time.Time
	var err error
	if task.Expiration != nil && *task.Expiration != "" {
		expiration, err = time.Parse(dateFormat, *task.Expiration)
		if err != nil || expiration.Before(time.Now().UTC()) {
			lpf(logh.Error, "expiration could not be parsed or is in the past: %+v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	} else {
		expiration = time.Now().UTC().Add(defaultExpirationDuration)
	}
	exp := expiration.Format(dateFormat)
	task.Expiration = &exp

	nu := uuid.New()
	task.UUID = &nu
	acpt := Accepted
	task.Status = &acpt

	// Create the directory used to hold output data for the task
	if _, err := os.Stat(task.Dir()); os.IsNotExist(err) {
		os.MkdirAll(task.Dir(), 0755)
	} else {
		lpf(logh.Error, "could not create directory: %s, error: %+v", task.Dir(), err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = telemetryKVS.Serialize(task.Key(), task)
	if err != nil {
		lpf(logh.Error, "%v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// TODO: provide REGEXP validation on Command and Shell.

	// Return the task UUID
	if aw, ok := w.(*authJWT.AuditWriter); ok {
		aw.Message = fmt.Sprintf("task create with UUID: %s", *task.UUID)
	}
	// Return a Task with only a UUID
	rtask := Task{UUID: task.UUID}
	b, err := json.Marshal(rtask)
	if err != nil {
		// Attempt to delete the task, since the client gets an error.
		telemetryKVS.Delete(task.Key())
		lpf(logh.Error, "json.Marshal error:%v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Schedule the task or error if there are no slots open to run another task
	select {
	case taskRun <- task.Key():
	case <-time.After(postScheduleLimit):
		w.WriteHeader(http.StatusTooManyRequests)
		return
	}

	// TODO: This needs to be an absolute path...
	// w.Header().Set("Location", strings.Replace(r.URL.RequestURI(), pathTask, pathStatus, -1))
	w.Header().Set("Location", path.Join(pathStatus, task.Key()))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	w.Write(b)
}

func taskPut(w http.ResponseWriter, r *http.Request) {
	task := Task{}
	if err := httph.BodyUnmarshal(w, r, &task); err != nil {
		lpf(logh.Error, "taskPut error:%v", err)
		return
	}
	if task.Cancel == nil ||
		(task.Cancel != nil && *task.Cancel == false) ||
		(task.Cancel != nil && *task.Cancel == true &&
			(task.Command != nil || task.Expiration != nil || task.Shell != nil || task.Status != nil ||
				task.UUID == nil || *task.UUID == uuid.Nil)) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	dtask := Task{}
	err := telemetryKVS.Deserialize(task.Key(), &dtask)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if dtask.UUID == nil || dtask.Status == nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	cncl := Canceling
	dtask.Status = &cncl
	err = telemetryKVS.Serialize(task.Key(), dtask)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	taskCancel <- task.Key()

	if aw, ok := w.(*authJWT.AuditWriter); ok {
		aw.Message = fmt.Sprintf("task status changed to %s with UUID: %s", Canceling, *task.UUID)
	}
	w.WriteHeader(http.StatusAccepted)
}
