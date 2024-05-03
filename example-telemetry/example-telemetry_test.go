package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/paulfdunn/go-helper/archiveh/ziph"
	"github.com/paulfdunn/go-helper/logh"
	"github.com/paulfdunn/go-helper/osh/runtimeh"
)

type expectedResponse struct {
	handlerFunc http.HandlerFunc
	httpMethod  string
	httpResonse int
	invalidKeys []string
	query       string
	task        *Task
}

func init() {
	t := testing.T{}
	testSetup(&t)
}

func Example_addTaskDirInclude() {
	// Use a fixed uuid so the test output is determinate.
	var uuid uuid.UUID
	uuid = [16]byte{0x00000000}
	// And use a fixed PersistentDirectory so the test output is determinate.
	fd := "fixed_dir"
	runtimeConfig.PersistentDirectory = &fd

	// Use {TASK_DIR_INCLUDE} with Command
	taskC := Task{Command: []string{"example.sh --task_dir={TASK_DIR_INCLUDE}"}, UUID: &uuid}
	taskC.addTaskDirInclude()
	fmt.Printf("%+v\n", taskC.Command)

	// Use {TASK_DIR_INCLUDE} with Shell
	taskS := Task{Shell: []string{"example.sh --task_dir={TASK_DIR_INCLUDE}"}, UUID: &uuid}
	taskS.addTaskDirInclude()
	fmt.Printf("%+v\n", taskS.Shell)

	// Output:
	// [example.sh --task_dir=fixed_dir/taskdata/00000000-0000-0000-0000-000000000000/include]
	// [example.sh --task_dir=fixed_dir/taskdata/00000000-0000-0000-0000-000000000000/include]
}

func ExampleTaskStatus() {
	var ts TaskStatus
	ts = 0
	fmt.Printf("TaskStatus[0]: %s\n", ts.String())
	ts = 1
	fmt.Printf("TaskStatus[1]: %s\n", ts.String())
	ts = 2
	fmt.Printf("TaskStatus[2]: %s\n", ts.String())
	ts = 3
	fmt.Printf("TaskStatus[3]: %s\n", ts.String())
	ts = 4
	fmt.Printf("TaskStatus[4]: %s\n", ts.String())
	ts = 5
	fmt.Printf("TaskStatus[5]: %s\n", ts.String())

	// Output:
	// TaskStatus[0]: Accepted
	// TaskStatus[1]: Canceled
	// TaskStatus[2]: Canceling
	// TaskStatus[3]: Completed
	// TaskStatus[4]: Expired
	// TaskStatus[5]: Running
}

// TestTaskEqual tests the Task.Equal function.
// Since the UUID is the key in the KVS, there is no need to test with it being nil.
// Expiration must also be not nil; don't test with nil.
func TestTaskEqual(t *testing.T) {
	// Validate that comparing a Task to itself, or one with the same UUID, passes and comparing
	// to a Task with a different UUID fails. Other values and slices are all nil.
	durDiff := time.Second * 30
	exp := "2024-03-25 18:30:00"
	uuid1 := uuid.New()
	uuid2 := uuid.New()
	t1 := Task{UUID: &uuid1, Expiration: &exp}
	t2 := Task{UUID: &uuid1, Expiration: &exp}
	t3 := Task{UUID: &uuid2, Expiration: &exp}
	if !t1.Equal(&t1, durDiff) || !t1.Equal(&t2, durDiff) || t1.Equal(&t3, durDiff) {
		t.Error("Task.Equal fail, just UUID")
	}

	// Validate that comparing a Task to itself, or one with the same UUID, passes and comparing
	// to a Task with a different UUID fails. Slices are all non-nil but empty.
	t1 = Task{UUID: &uuid1, Cancel: nil, Command: []string{}, Expiration: &exp, ProcessCommand: []string{}, ProcessShell: []string{}, ProcessZip: []string{}, Shell: []string{}, Status: nil}
	t2 = Task{UUID: &uuid1, Cancel: nil, Command: []string{}, Expiration: &exp, ProcessCommand: []string{}, ProcessShell: []string{}, ProcessZip: []string{}, Shell: []string{}, Status: nil}
	t3 = Task{UUID: &uuid2, Cancel: nil, Command: []string{}, Expiration: &exp, ProcessCommand: []string{}, ProcessShell: []string{}, ProcessZip: []string{}, Shell: []string{}, Status: nil}
	if !t1.Equal(&t1, durDiff) || !t1.Equal(&t2, durDiff) || t1.Equal(&t3, durDiff) {
		t.Error("Task.Equal fail, UUID and empty Command/Shell")
	}

	// Test each value field, all with the same UUID.
	// test Cancel
	tr := true
	fl := false
	t1 = Task{UUID: &uuid1, Cancel: &tr, Command: []string{}, Expiration: &exp, ProcessCommand: []string{}, ProcessShell: []string{}, ProcessZip: []string{}, Shell: []string{}, Status: nil}
	t2 = Task{UUID: &uuid1, Cancel: &tr, Command: []string{}, Expiration: &exp, ProcessCommand: []string{}, ProcessShell: []string{}, ProcessZip: []string{}, Shell: []string{}, Status: nil}
	t3 = Task{UUID: &uuid1, Cancel: &fl, Command: []string{}, Expiration: &exp, ProcessCommand: []string{}, ProcessShell: []string{}, ProcessZip: []string{}, Shell: []string{}, Status: nil}
	if !t1.Equal(&t1, durDiff) || !t1.Equal(&t2, durDiff) || t1.Equal(&t3, durDiff) {
		t.Error("Task.Equal fail, UUID and Cancel")
	}
	// test Command
	sl1 := "some_value"
	sl2 := "Some_value"
	t1 = Task{UUID: &uuid1, Cancel: nil, Command: []string{sl1}, Expiration: &exp, ProcessCommand: []string{}, ProcessShell: []string{}, ProcessZip: []string{}, Shell: []string{}, Status: nil}
	t2 = Task{UUID: &uuid1, Cancel: nil, Command: []string{sl1}, Expiration: &exp, ProcessCommand: []string{}, ProcessShell: []string{}, ProcessZip: []string{}, Shell: []string{}, Status: nil}
	t3 = Task{UUID: &uuid1, Cancel: nil, Command: []string{sl2}, Expiration: &exp, ProcessCommand: []string{}, ProcessShell: []string{}, ProcessZip: []string{}, Shell: []string{}, Status: nil}
	if !t1.Equal(&t1, durDiff) || !t1.Equal(&t2, durDiff) || t1.Equal(&t3, durDiff) {
		t.Error("Task.Equal fail, UUID and Cancel")
	}
	// test File
	sl1 = "some_value"
	sl2 = "Some_value"
	t1 = Task{UUID: &uuid1, Cancel: nil, Command: []string{sl1}, Expiration: &exp, File: []string{}, ProcessShell: []string{}, ProcessZip: []string{}, Shell: []string{}, Status: nil}
	t2 = Task{UUID: &uuid1, Cancel: nil, Command: []string{sl1}, Expiration: &exp, File: []string{}, ProcessShell: []string{}, ProcessZip: []string{}, Shell: []string{}, Status: nil}
	t3 = Task{UUID: &uuid1, Cancel: nil, Command: []string{sl2}, Expiration: &exp, File: []string{}, ProcessShell: []string{}, ProcessZip: []string{}, Shell: []string{}, Status: nil}
	if !t1.Equal(&t1, durDiff) || !t1.Equal(&t2, durDiff) || t1.Equal(&t3, durDiff) {
		t.Error("Task.Equal fail, UUID and File")
	}
	// test ProcessCommand
	t1 = Task{UUID: &uuid1, Cancel: nil, Command: []string{}, Expiration: &exp, ProcessCommand: []string{sl1}, ProcessShell: []string{}, ProcessZip: []string{}, Shell: []string{}, Status: nil}
	t2 = Task{UUID: &uuid1, Cancel: nil, Command: []string{}, Expiration: &exp, ProcessCommand: []string{sl1}, ProcessShell: []string{}, ProcessZip: []string{}, Shell: []string{}, Status: nil}
	t3 = Task{UUID: &uuid1, Cancel: nil, Command: []string{}, Expiration: &exp, ProcessCommand: []string{sl2}, ProcessShell: []string{}, ProcessZip: []string{}, Shell: []string{}, Status: nil}
	if !t1.Equal(&t1, durDiff) || !t1.Equal(&t2, durDiff) || t1.Equal(&t3, durDiff) {
		t.Error("Task.Equal fail, UUID and ProcessCommand")
	}
	// test ProcessShell
	t1 = Task{UUID: &uuid1, Cancel: nil, Command: []string{}, Expiration: &exp, ProcessCommand: []string{}, ProcessShell: []string{sl1}, ProcessZip: []string{}, Shell: []string{}, Status: nil}
	t2 = Task{UUID: &uuid1, Cancel: nil, Command: []string{}, Expiration: &exp, ProcessCommand: []string{}, ProcessShell: []string{sl1}, ProcessZip: []string{}, Shell: []string{}, Status: nil}
	t3 = Task{UUID: &uuid1, Cancel: nil, Command: []string{}, Expiration: &exp, ProcessCommand: []string{}, ProcessShell: []string{sl2}, ProcessZip: []string{}, Shell: []string{}, Status: nil}
	if !t1.Equal(&t1, durDiff) || !t1.Equal(&t2, durDiff) || t1.Equal(&t3, durDiff) {
		t.Error("Task.Equal fail, UUID and ProcessShell")
	}
	// test ProcessZip
	t1 = Task{UUID: &uuid1, Cancel: nil, Command: []string{}, Expiration: &exp, ProcessCommand: []string{}, ProcessShell: []string{}, ProcessZip: []string{sl1}, Shell: []string{}, Status: nil}
	t2 = Task{UUID: &uuid1, Cancel: nil, Command: []string{}, Expiration: &exp, ProcessCommand: []string{}, ProcessShell: []string{}, ProcessZip: []string{sl1}, Shell: []string{}, Status: nil}
	t3 = Task{UUID: &uuid1, Cancel: nil, Command: []string{}, Expiration: &exp, ProcessCommand: []string{}, ProcessShell: []string{}, ProcessZip: []string{sl2}, Shell: []string{}, Status: nil}
	if !t1.Equal(&t1, durDiff) || !t1.Equal(&t2, durDiff) || t1.Equal(&t3, durDiff) {
		t.Error("Task.Equal fail, UUID and ProcessZip")
	}
	// test Shell
	t1 = Task{UUID: &uuid1, Cancel: nil, Command: []string{}, Expiration: &exp, ProcessCommand: []string{}, ProcessShell: []string{}, ProcessZip: []string{}, Shell: []string{sl1}, Status: nil}
	t2 = Task{UUID: &uuid1, Cancel: nil, Command: []string{}, Expiration: &exp, ProcessCommand: []string{}, ProcessShell: []string{}, ProcessZip: []string{}, Shell: []string{sl1}, Status: nil}
	t3 = Task{UUID: &uuid1, Cancel: nil, Command: []string{}, Expiration: &exp, ProcessCommand: []string{}, ProcessShell: []string{}, ProcessZip: []string{}, Shell: []string{sl2}, Status: nil}
	if !t1.Equal(&t1, durDiff) || !t1.Equal(&t2, durDiff) || t1.Equal(&t3, durDiff) {
		t.Error("Task.Equal fail, UUID and Shell")
	}
	// test Status
	accptd := Accepted
	rnng := Running
	t1 = Task{UUID: &uuid1, Cancel: nil, Command: []string{}, Expiration: &exp, ProcessCommand: []string{}, ProcessShell: []string{}, ProcessZip: []string{}, Shell: []string{}, Status: &accptd}
	t2 = Task{UUID: &uuid1, Cancel: nil, Command: []string{}, Expiration: &exp, ProcessCommand: []string{}, ProcessShell: []string{}, ProcessZip: []string{}, Shell: []string{}, Status: &accptd}
	t3 = Task{UUID: &uuid1, Cancel: nil, Command: []string{}, Expiration: &exp, ProcessCommand: []string{}, ProcessShell: []string{}, ProcessZip: []string{}, Shell: []string{}, Status: &rnng}
	if !t1.Equal(&t1, durDiff) || !t1.Equal(&t2, durDiff) || t1.Equal(&t3, durDiff) {
		t.Error("Task.Equal fail, UUID and Status")
	}

	// Validate that allowedExpirationDifference is working properly by testing: the same expiration, a value
	// one second within range, and one second outside the range.
	exp1 := "2024-03-25 18:31:00"
	exp2 := "2024-03-25 18:31:29"
	// Set 1 second out of the allowed range and fail if equal.
	exp3 := "2024-03-25 18:31:31"
	t1 = Task{UUID: &uuid1, Command: []string{}, Expiration: &exp1, Shell: []string{}}
	t2 = Task{UUID: &uuid1, Command: []string{}, Expiration: &exp2, Shell: []string{}}
	t3 = Task{UUID: &uuid1, Command: []string{}, Expiration: &exp3, Shell: []string{}}
	if !t1.Equal(&t1, durDiff) || !t1.Equal(&t2, durDiff) || t1.Equal(&t3, durDiff) {
		t.Error("Task.Equal fails with duration issues.")
	}
}

// TestExpectedResponse are tests to validate API requirements for parameters. An API call is made, and
// the resp.StatusCode is validated against what is expected.
func TestExpectedResponse(t *testing.T) {
	invalidUUID := uuid.New()
	validUUID := uuid.New()
	task := Task{UUID: &validUUID}
	err := telemetryKVS.Serialize(task.Key(), task)
	if err != nil {
		t.Errorf("Serialize error: %+v", err)
	}

	expectedResponses := []expectedResponse{}

	// http.MethoDelete tests
	// negative tests - invalid UUID
	task1 := Task{UUID: &invalidUUID}
	expectedResponses = append(expectedResponses, expectedResponse{handlerTask, http.MethodDelete, http.StatusBadRequest, nil, "?uuid=" + invalidUUID.String(), &task1})
	// positive tests - for a valid UUID, the Task status must be one of Canceled, Completed, or Expired. So
	// create valid tasks and test against those.
	for i := Accepted; i <= Running; i++ {
		vu := uuid.New()
		st := i
		// Serialize a task with status, but Status must be empty for the request.
		stask := Task{Status: &st, UUID: &vu}
		if err := telemetryKVS.Serialize(stask.Key(), stask); err != nil {
			t.Errorf("Serialize error: %+v", err)
		}
		var expectedStatus int
		switch i {
		case Canceled, Completed, Expired:
			expectedStatus = http.StatusNoContent
		default:
			expectedStatus = http.StatusBadRequest
		}
		expectedResponses = append(expectedResponses, expectedResponse{handlerTask, http.MethodDelete, expectedStatus, nil, "?uuid=" + vu.String(), nil})
	}

	// http.MethodGet tests
	// negative tests - invalid UUID
	task4 := Task{UUID: &invalidUUID}
	expectedResponses = append(expectedResponses, expectedResponse{handlerTask, http.MethodGet, http.StatusBadRequest, nil, "?uuid=" + invalidUUID.String(), &task4})

	// http.MethodPost tests
	// Post with a UUID is not valid
	task5 := Task{UUID: &validUUID}
	expectedResponses = append(expectedResponses, expectedResponse{handlerTask, http.MethodPost, http.StatusBadRequest, nil, "", &task5})
	task6 := Task{}
	ik := []string{taskKeyCancel, taskKeyProcessCommand, taskKeyProcessError,
		taskKeyProcessShell, taskKeyProcessZip, taskKeyStatus}
	expectedResponses = append(expectedResponses, expectedResponse{handlerTask, http.MethodPost, http.StatusBadRequest, ik, "", &task6})

	// http.MethodPut tests
	// PUT is only valid with both Cancel==true and a valid UUID
	tr := true
	task7 := Task{UUID: &validUUID}
	expectedResponses = append(expectedResponses, expectedResponse{handlerTask, http.MethodPut, http.StatusBadRequest, nil, "", &task7})
	task8 := Task{UUID: &validUUID, Cancel: &tr}
	expectedResponses = append(expectedResponses, expectedResponse{handlerTask, http.MethodPut, http.StatusAccepted, nil, "", &task8})
	task9 := Task{UUID: &validUUID}
	ik = []string{taskKeyCommand, taskKeyFile, taskKeyExpiration, taskKeyProcessCommand, taskKeyProcessError,
		taskKeyProcessShell, taskKeyProcessZip, taskKeyShell, taskKeyStatus}
	expectedResponses = append(expectedResponses, expectedResponse{handlerTask, http.MethodPut, http.StatusBadRequest, ik, "", &task9})

	for i, er := range expectedResponses {
		if i == 11 {
			fmt.Println("")
		}
		if er.invalidKeys != nil {
			for _, invalidKey := range er.invalidKeys {
				er.task.setKey(invalidKey, true)
				er.test(t, i)
				er.task.setKey(invalidKey, false)
			}
		} else {
			er.test(t, i)
		}
	}
}

// TestStatus POSTs several tasks, and validates it can get a single status with a query string and
// that it can get all status for all created tasks.
func TestStatus(t *testing.T) {
	testServerStatus := httptest.NewServer(http.HandlerFunc(handlerStatus))
	defer testServerStatus.Close()

	if err := clearTelemetryKVS(t); err != nil {
		t.Errorf("clearTelemetryKVS error:%+v", err)
	}

	// Make some tasks for testing and POST them
	tasks := make([]*Task, 4)
	for i := range tasks {
		task := Task{}
		rtask, err := testTaskPost(t, task)
		if err != nil {
			t.Errorf("could not POST task: %+v", err)
			return
		}
		tasks[i] = rtask
	}

	rtasks := []Task{}
	if err := getAndUnmarshal(t, testServerStatus.URL, &rtasks); err != nil {
		t.Errorf("getAndUnmarshal error: %+v", err)
	}

	if len(rtasks) != len(tasks) {
		t.Errorf("status returned incorrect data")
	}

	// Use query strings to successively ask for tasks
	for i := range tasks {
		queries := make([]string, i+1)
		for j := 0; j <= i; j++ {
			queries[j] = fmt.Sprintf("%s=%s", queryParamUUID, tasks[j].Key())
		}
		query := "/?" + strings.Join(queries, "&")

		rtasks = []Task{}
		if err := getAndUnmarshal(t, testServerStatus.URL+query, &rtasks); err != nil {
			t.Errorf("getAndUnmarshal error: %+v", err)
		}

		if len(rtasks) != i+1 {
			t.Errorf("status returned incorrect data")
		}
	}
}

// TestTaskPost verifies POSTs with default and specified expirations
// run and complete. Verification is done by reading the KVS
func TestTaskPost(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(handlerTask))
	defer testServer.Close()

	// Loop0 - POST a task with no expiration and validate it is set to the default.
	// Loop1 - POST a task with an expiration that is not default.
	cmd := []string{"ls"}
	for i := 0; i <= 1; i++ {
		var task Task
		// Pick an expiration that is NOT default, and will never be default.
		expOffset := time.Duration(defaultExpirationDuration * 2)
		task = Task{Shell: cmd}
		if i == 1 {
			es := time.Now().UTC().Add(expOffset).Format(dateFormat)
			task = Task{Expiration: &es, Shell: cmd}
		}

		rtask, err := testTaskPost(t, task)
		if err != nil {
			t.Errorf("could not POST task: %+v", err)
			return
		}

		// Sleep long enough to make sure the task has been processed by taskRunner
		time.Sleep(taskRunnerCycleTime * 5)

		// Add the UUID, Completed Status, and Expiration to the sent task and compare to the deserialized task
		task.UUID = rtask.UUID
		sts := Completed
		task.Status = &sts
		task.ProcessShell = cmd
		exp := time.Now().UTC().Add(defaultExpirationDuration).Format(dateFormat)
		if i == 1 {
			exp = time.Now().UTC().Add(expOffset).Format(dateFormat)
		}
		task.Expiration = &exp
		dtask := Task{}
		err = telemetryKVS.Deserialize(rtask.Key(), &dtask)
		if err != nil {
			t.Errorf("Could not deserialize task: %+v", err)
			return
		}
		dtask.ProcessZip = nil
		if !dtask.Equal(&task, time.Second*20) {
			t.Errorf("Sent and deserialized tasks are not equal, \ntask: %+v, \ndtask: %+v", task, dtask)
			return
		}
	}
}

// TestTaskPostAndDelete does a POST to create a task, updates the status to Completed, then deletes that task.
func TestTaskPostAndDelete(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(handlerTask))
	defer testServer.Close()

	task := Task{}
	rtask, err := testTaskPost(t, task)
	if err != nil {
		t.Errorf("could not POST task: %+v", err)
		return
	}

	// Sleep long enough to make sure the task has been processed by taskRunner
	time.Sleep(taskRunnerCycleTime * 5)

	err = telemetryKVS.Deserialize(rtask.Key(), &rtask)
	if err != nil {
		t.Errorf("Could not deserialize task: %+v", err)
		return
	}
	st := Completed
	rtask.Status = &st
	err = telemetryKVS.Serialize(rtask.Key(), rtask)
	if err != nil {
		t.Errorf("Serialize error: %+v", err)
	}
	client := &http.Client{}
	req, err := http.NewRequest(http.MethodDelete, testServer.URL+"?uuid="+rtask.UUID.String(), nil)
	if err != nil {
		t.Errorf("NewRequest error: %v", err)
		return
	}
	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != http.StatusNoContent {
		t.Errorf("DELETE did not return proper status: %d", resp.StatusCode)
		return
	}
}

// TestTaskPostAndCancel POSTs a new task, validates it status gets changed to Completed, and validates canceling that task
func TestTaskPostAndCancel(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(handlerTask))
	defer testServer.Close()

	task := Task{Shell: []string{"ls -alt"}}
	rtask, err := testTaskPost(t, task)
	if err != nil {
		t.Errorf("could not POST task: %+v", err)
		return
	}

	// Sleep long enough to make sure the task has been processed by taskRunner
	time.Sleep(taskRunnerCycleTime * 5)

	// Validate the status is Completed
	dtask := Task{}
	err = telemetryKVS.Deserialize(rtask.Key(), &dtask)
	if err != nil {
		t.Errorf("Could not deserialize task: %+v", err)
		return
	}
	if dtask.Status == nil || *dtask.Status != Completed {
		t.Errorf("task status is not Completed, task: %+v", dtask)
		return
	}

	// Cancel the task
	tr := true
	rtask.Cancel = &tr
	rtaskBytes, err := json.Marshal(*rtask)
	if err != nil {
		t.Errorf("Marshal error: %v", err)
		return
	}
	err = requestAndDeserialize(t, http.MethodPut, testServer.URL, rtaskBytes, rtask.Key(), &dtask)
	if err != nil {
		t.Errorf("could not deserialize task: %+v", err)
		return
	}

	// Sleep long enough to make sure the task has been processed by taskRunner
	time.Sleep(taskRunnerCycleTime * 5)

	// Validate the status is Canceled
	err = telemetryKVS.Deserialize(rtask.Key(), &dtask)
	if err != nil {
		t.Errorf("Could not deserialize task: %+v", err)
		return
	}
	if dtask.Status == nil || *dtask.Status != Canceled {
		t.Errorf("task status is not Canceled, task: %+v", dtask)
		return
	}
}

// Post new task, poll status until completed, download file and validate.
// Test twice: loop 0 tests no FileModifiedSeconds, loop 1 tests FileModifiedSeconds
// with a short value to filter out the test file.
func TestRoundTrip(t *testing.T) {
	fms := 1
	fileModifiedSeconds := []*int{nil, &fms}
	for i := 0; i <= 1; i++ {
		cmd := "ls -alt"
		cmdInclude := "echo 'hello world' > {TASK_DIR_INCLUDE}/helloworld.txt"
		// fileTestWildcard needs to match exactly ONE file and that needs to be fileTest
		fileTestWildcard := "./example-telemetry.g*"
		fileTest := "./example-telemetry.go"
		task := Task{File: []string{fileTestWildcard}, Shell: []string{cmd,
			cmdInclude}, FileModifiedSeconds: fileModifiedSeconds[i]}
		rtask, err := testTaskPost(t, task)
		if err != nil {
			t.Errorf("could not POST task: %+v", err)
			return
		}

		var expectedFiles []string
		cmdInclude = strings.ReplaceAll(cmdInclude, taskDirIncludeMarker, rtask.DirInclude())
		switch i {
		case 0:
			expectedFiles = []string{
				filenameFromCommand(cmd) + stderrFileSuffix,
				filenameFromCommand(cmd) + stdoutFileSuffix,
				filepath.Base(fileTest),
				filenameFromCommand(cmdInclude) + stderrFileSuffix,
				filenameFromCommand(cmdInclude) + stdoutFileSuffix,
			}
		case 1:
			expectedFiles = []string{filenameFromCommand(cmd) + stderrFileSuffix,
				filenameFromCommand(cmd) + stdoutFileSuffix}
		}

		testServerStatus := httptest.NewServer(http.HandlerFunc(handlerStatus))
		defer testServerStatus.Close()
		testServerTask := httptest.NewServer(http.HandlerFunc(handlerTask))
		defer testServerTask.Close()

		// Sleep long enough for files to age out when testing FileModifiedSeconds
		time.Sleep(time.Duration(fms) * time.Second)

		// Use query strings to ask for task and wait until done.
		for {
			query := "/?" + fmt.Sprintf("%s=%s", queryParamUUID, rtask.Key())

			stasks := []Task{}
			if err := getAndUnmarshal(t, testServerStatus.URL+query, &stasks); err != nil {
				t.Errorf("getAndUnmarshal error: %+v", err)
			}

			if len(stasks) != 1 {
				t.Errorf("status returned incorrect data")
			}
			if stasks[0].Status != nil && *stasks[0].Status == Completed {
				break
			}
		}

		// Get the zip file
		outputZipFilepath := filepath.Join(t.TempDir(), "test.zip")
		out, err := os.Create(outputZipFilepath)
		if err != nil {
			t.Errorf("file create: %+v", err)
		}
		defer out.Close()
		client := &http.Client{}
		query := fmt.Sprintf(`?uuid=%s`, rtask.UUID.String())
		req, err := http.NewRequest(http.MethodGet, testServerTask.URL+query, nil)
		if err != nil {
			t.Errorf("NewRequest error: %v", err)
			return
		}
		resp, err := client.Do(req)
		if err != nil || resp.StatusCode != http.StatusOK {
			t.Errorf("request did not return proper status: %d", resp.StatusCode)
			return
		}
		defer resp.Body.Close()
		_, err = io.Copy(out, resp.Body)
		if err != nil {
			t.Errorf("file copy: %+v", err)
		}
		out.Close()

		// Unzip the returned file and find files of the correct name.
		_, processedPaths, errs := ziph.AsyncUnzip(outputZipFilepath, t.TempDir(), len(expectedFiles), 0755)
		pathCount := 0
		errCount := 0
		for {
			noMessage := false
			select {
			case pp, ok := <-processedPaths:
				if ok {
					if slices.Contains(expectedFiles, filepath.Base(pp)) {
						pathCount++
					}
					lpf(logh.Info, "AsyncUnzip processed path: %s\n", pp)
				} else {
					lpf(logh.Info, "AsyncUnzip processedPaths is nil")
					processedPaths = nil
				}
			case err, ok := <-errs:
				if ok {
					errCount++
					lpf(logh.Error, "AsyncUnzip error: %v\n", err)
				} else {
					lpf(logh.Info, "AsyncUnzip error channel is nil")
					errs = nil
				}
			default:
				noMessage = true
			}

			if noMessage {
				if processedPaths == nil && errs == nil {
					lpf(logh.Info, "AsyncUnzip is done.")
					break
				}
				time.Sleep(time.Millisecond)
			}
		}
		if pathCount != len(expectedFiles) {
			t.Errorf("wrong number of files, got %d, expected %d", pathCount, len(expectedFiles))
		}
	}
}

func (er expectedResponse) test(t *testing.T, i int) {
	testServer := httptest.NewServer(http.HandlerFunc(er.handlerFunc))
	defer testServer.Close()

	taskBytes, err := json.Marshal(er.task)
	if err != nil {
		t.Errorf("Marshal error: %v", err)
		return
	}
	client := &http.Client{}
	req, err := http.NewRequest(er.httpMethod, testServer.URL+er.query, bytes.NewBuffer(taskBytes))
	if err != nil {
		t.Errorf("NewRequest error: %v", err)
		return
	}
	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != er.httpResonse {
		t.Errorf("i: %d, method: %s, task: %+v, did not return proper status, reveived: %d, expected status: %d", i, er.httpMethod, er.task, resp.StatusCode, er.httpResonse)
		return
	}
}

// setKey will either set or clear a key for testing.
func (tsk *Task) setKey(key string, set bool) {
	if set {
		switch key {
		case taskKeyCancel:
			tr := true
			tsk.Cancel = &tr
		case taskKeyCommand:
			tsk.Command = []string{"nothing"}
		case taskKeyFile:
			tsk.File = []string{"nothing"}
		case taskKeyProcessError:
			tsk.ProcessError = []string{"nothing"}
		case taskKeyExpiration:
			exp := time.Now().Add(time.Minute).Format(dateFormat)
			tsk.Expiration = &exp
		case taskKeyProcessCommand:
			tsk.ProcessCommand = []string{"nothing"}
		case taskKeyProcessShell:
			tsk.ProcessShell = []string{"nothing"}
		case taskKeyProcessZip:
			tsk.ProcessZip = []string{"nothing"}
		case taskKeyShell:
			tsk.Shell = []string{"nothing"}
		case taskKeyStatus:
			accptd := Accepted
			tsk.Status = &accptd
		}
	} else {
		switch key {
		case taskKeyCancel:
			tsk.Cancel = nil
		case taskKeyCommand:
			tsk.Command = nil
		case taskKeyFile:
			tsk.File = nil
		case taskKeyProcessError:
			tsk.ProcessError = nil
		case taskKeyExpiration:
			tsk.Expiration = nil
		case taskKeyProcessCommand:
			tsk.ProcessCommand = nil
		case taskKeyProcessShell:
			tsk.ProcessShell = nil
		case taskKeyProcessZip:
			tsk.ProcessZip = nil
		case taskKeyShell:
			tsk.Shell = nil
		case taskKeyStatus:
			tsk.Status = nil
		}
	}
}

func clearTelemetryKVS(t *testing.T) error {
	keys, err := telemetryKVS.Keys()
	if err != nil {
		t.Errorf("could not get keys: %+v", err)
		return err
	}
	for _, key := range keys {
		_, err := telemetryKVS.Delete(key)
		if err != nil {
			t.Errorf("could not delete key: %+v", err)
			return err
		}
	}
	return nil
}

func getAndUnmarshal(t *testing.T, URL string, obj interface{}) error {
	resp, err := http.Get(URL)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Errorf("did not return proper status: %d", resp.StatusCode)
		return err
	}
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Errorf("ReadAll error: %v", err)
		return err
	}
	resp.Body.Close()
	err = json.Unmarshal(bodyBytes, obj)
	if err != nil {
		t.Errorf("could not unmarshal tasks: %+v", err)
		return err
	}
	return nil
}

// requestAndDeserialize will make a request using httpMethod, sending inputBytes, then deserialize key into obj.
func requestAndDeserialize(t *testing.T, httpMethod string, URL string, inputBytes []byte, key string, obj interface{}) error {
	client := &http.Client{}
	req, err := http.NewRequest(httpMethod, URL, bytes.NewBuffer(inputBytes))
	if err != nil {
		t.Errorf("NewRequest error: %v", err)
		return err
	}
	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != http.StatusAccepted {
		t.Errorf("request did not return proper status: %d", resp.StatusCode)
		return err
	}
	err = telemetryKVS.Deserialize(key, obj)
	if err != nil {
		t.Errorf("could not deserialize obj: %+v", err)
		return err
	}
	return nil
}

// testTaskPost is a helper function to POST a Task and return the returned Task.
func testTaskPost(t *testing.T, task Task) (*Task, error) {
	testServer := httptest.NewServer(http.HandlerFunc(handlerTask))
	defer testServer.Close()

	taskBytes, err := json.Marshal(task)
	if err != nil {
		t.Errorf("marshal error: %v", err)
		return nil, err
	}
	resp, err := http.Post(testServer.URL, "application/json", bytes.NewBuffer(taskBytes))
	if err != nil || resp.StatusCode != http.StatusCreated {
		t.Errorf("POST method did not return proper status: %d", resp.StatusCode)
		return nil, fmt.Errorf("%+v", err)
	}
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Errorf("ReadAll error: %v", err)
		return nil, err
	}
	resp.Body.Close()
	// The response is just the UUID
	rtask := Task{}
	err = json.Unmarshal(bodyBytes, &rtask)
	if err != nil {
		t.Errorf("could not unmarshal task: %+v", err)
		return nil, err
	}
	return &rtask, nil
}

func testSetup(t *testing.T) {
	// flag.Parse is called as part of main and thus it cannot be called in init()
	// go func() {
	// 	main()
	// }()

	// log to STDOUT
	err := logh.New("", "", logh.DefaultLevels, logh.Info, logh.DefaultFlags, 0, 0)
	if err != nil {
		log.Fatalf("fatal: %s error creating audit log, error: %v", runtimeh.SourceInfo(), err)
	}
	lp = logh.Map[""].Println
	lpf = logh.Map[""].Printf
	lp(logh.Info, "testSetup")

	tempDir := t.TempDir()
	lpf(logh.Info, "tempDir: %s\n", tempDir)
	runtimeConfig.PersistentDirectory = &tempDir

	// Every call to TempDir returns a unique directory; there is no need to remove files.
	initializeKVS(tempDir, appName+telemetryFileSuffix)
	maxTasks = 500
	initializeTaskInfrastructure()
}
