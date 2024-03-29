package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/paulfdunn/go-helper/logh"
	"github.com/paulfdunn/go-helper/osh/runtimeh"
)

type expectedResponse struct {
	httpMethod  string
	task        Task
	httpResonse int
	handlerFunc http.HandlerFunc
}

func init() {
	t := testing.T{}
	testSetup(&t)
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
func TestTaskEqual(t *testing.T) {
	// Since the UUID is the key in the KVS, there is no need to test with it being nil.
	// Expiration must also be not nil; don't test with nil.
	durDiff := time.Second * 30
	exp := "2024-03-25 18:30:00"
	uuid1 := uuid.New()
	uuid2 := uuid.New()
	t1 := Task{UUID: &uuid1, Expiration: &exp}
	t2 := Task{UUID: &uuid1, Expiration: &exp}
	t3 := Task{UUID: &uuid2, Expiration: &exp}
	if !t1.Equal(&t1, durDiff) || !t1.Equal(&t2, durDiff) || t1.Equal(&t3, durDiff) {
		t.Error("Task.Equal fail,durDiffs with just UUID")
	}

	t4 := Task{UUID: &uuid1, Command: []string{}, Expiration: &exp, Shell: []string{}}
	t5 := Task{UUID: &uuid1, Command: []string{}, Expiration: &exp, Shell: []string{}}
	t6 := Task{UUID: &uuid2, Command: []string{}, Expiration: &exp, Shell: []string{}}
	if !t4.Equal(&t4, durDiff) || !t4.Equal(&t5, durDiff) || t4.Equal(&t6, durDiff) {
		t.Error("Task.Equal fail,durDiffs with UUID and empty Command/Shell")
	}

	t7 := Task{UUID: &uuid1, Command: []string{"some_command", "other_command"}, Expiration: &exp, Shell: []string{"ls -al"}}
	t8 := Task{UUID: &uuid1, Command: []string{"some_command", "other_command"}, Expiration: &exp, Shell: []string{"ls -al"}}
	t9 := Task{UUID: &uuid2, Command: []string{"Some_command", "Other_command"}, Expiration: &exp, Shell: []string{"ls -AL"}}
	t10 := Task{UUID: &uuid2, Command: []string{"Some_command"}, Expiration: &exp, Shell: []string{"ls -AL"}}
	if !t7.Equal(&t7, durDiff) || !t7.Equal(&t8, durDiff) || t7.Equal(&t9, durDiff) || t7.Equal(&t10, durDiff) {
		t.Error("Task.Equal fail,durDiffs with UUID and empty Command/Shell")
	}

	exp1 := "2024-03-25 18:31:00"
	exp2 := "2024-03-25 18:31:29"
	// Set 1 second out of the allowed range and fail if equal.
	exp3 := "2024-03-25 18:31:31"
	t11 := Task{UUID: &uuid1, Command: []string{}, Expiration: &exp1, Shell: []string{}}
	t12 := Task{UUID: &uuid1, Command: []string{}, Expiration: &exp2, Shell: []string{}}
	t13 := Task{UUID: &uuid1, Command: []string{}, Expiration: &exp3, Shell: []string{}}
	if !t11.Equal(&t11, durDiff) || !t11.Equal(&t12, durDiff) || t11.Equal(&t13, durDiff) {
		t.Error("Task.Equal fails with duration issues.")
	}
}

// TestExpectedResponse are tests to validate API requirements for parameters. A task is sent, and
// the resp.StatusCode is validated against what is expected.
func TestExpectedResponse(t *testing.T) {
	cancelTrue := true
	cancelFalse := false
	command := []string{"cmd"}
	exp := time.Now().Add(time.Minute).Format(dateFormat)
	status := Accepted
	shell := []string{"shell"}
	invalidUUID := uuid.New()
	validUUID := uuid.New()
	task := Task{UUID: &validUUID}
	err := telemetryKVS.Serialize(task.Key(), task)
	if err != nil {
		t.Errorf("Serialize error: %+v", err)
	}

	expectedResponses := []expectedResponse{}

	// http.MethoDelete tests
	// Delete can only provide the UUID
	task = Task{}
	expectedResponses = append(expectedResponses, expectedResponse{http.MethodDelete, task, http.StatusBadRequest, handlerTask})
	task = Task{UUID: &invalidUUID}
	expectedResponses = append(expectedResponses, expectedResponse{http.MethodDelete, task, http.StatusBadRequest, handlerTask})
	task = Task{Cancel: &cancelTrue, UUID: &validUUID}
	expectedResponses = append(expectedResponses, expectedResponse{http.MethodDelete, task, http.StatusBadRequest, handlerTask})
	task = Task{Command: command, UUID: &validUUID}
	expectedResponses = append(expectedResponses, expectedResponse{http.MethodDelete, task, http.StatusBadRequest, handlerTask})
	// Make sure the time is in the future.
	task = Task{Expiration: &exp, UUID: &validUUID}
	expectedResponses = append(expectedResponses, expectedResponse{http.MethodDelete, task, http.StatusBadRequest, handlerTask})
	task = Task{Status: &status, UUID: &validUUID}
	expectedResponses = append(expectedResponses, expectedResponse{http.MethodDelete, task, http.StatusBadRequest, handlerTask})
	task = Task{Shell: shell, UUID: &validUUID}
	expectedResponses = append(expectedResponses, expectedResponse{http.MethodDelete, task, http.StatusBadRequest, handlerTask})
	// Task status must be one of Canceled, Completed, or Expired
	for i := Accepted; i <= Running; i++ {
		vu := uuid.New()
		st := i
		// Serialize a task with status, but Status must be empty for the request.
		stask := Task{Status: &st, UUID: &vu}
		telemetryKVS.Serialize(stask.Key(), stask)
		if err != nil {
			t.Errorf("Serialize error: %+v", err)
		}
		task := Task{UUID: &vu}
		var status int
		switch i {
		case Canceled, Completed, Expired:
			status = http.StatusNoContent
		default:
			status = http.StatusBadRequest
		}
		expectedResponses = append(expectedResponses, expectedResponse{http.MethodDelete, task, status, handlerTask})
	}

	// http.MethodGet tests
	// Get with invalid UUID
	task = Task{UUID: &invalidUUID}
	expectedResponses = append(expectedResponses, expectedResponse{http.MethodGet, task, http.StatusBadRequest, handlerTask})

	// http.MethodPost tests
	// Post with a UUID is not valid
	task = Task{UUID: &validUUID}
	expectedResponses = append(expectedResponses, expectedResponse{http.MethodPost, task, http.StatusBadRequest, handlerTask})
	// POST with a Status (loop 0) or Cancel (loop1) are not valid
	task = Task{Status: &status}
	expectedResponses = append(expectedResponses, expectedResponse{http.MethodPost, task, http.StatusBadRequest, handlerTask})
	task = Task{Cancel: &cancelTrue}
	expectedResponses = append(expectedResponses, expectedResponse{http.MethodPost, task, http.StatusBadRequest, handlerTask})
	// POST with a expiration in the past is not valid.
	task = Task{Expiration: &exp}
	expectedResponses = append(expectedResponses, expectedResponse{http.MethodPost, task, http.StatusBadRequest, handlerTask})

	// http.MethodPut tests
	// PUT without both Cancel and UUID, with an invalid UUID, or with a Command, Expiration,
	// Shell, or Status are not valid.
	task = Task{Cancel: &cancelTrue}
	expectedResponses = append(expectedResponses, expectedResponse{http.MethodPut, task, http.StatusBadRequest, handlerTask})
	task = Task{UUID: &validUUID}
	expectedResponses = append(expectedResponses, expectedResponse{http.MethodPut, task, http.StatusBadRequest, handlerTask})
	task = Task{Cancel: &cancelTrue, Command: command, UUID: &validUUID}
	expectedResponses = append(expectedResponses, expectedResponse{http.MethodPut, task, http.StatusBadRequest, handlerTask})
	// Make sure the time is in the future.
	exp = time.Now().Add(time.Minute).Format(dateFormat)
	task = Task{Cancel: &cancelTrue, Expiration: &exp, UUID: &validUUID}
	expectedResponses = append(expectedResponses, expectedResponse{http.MethodPut, task, http.StatusBadRequest, handlerTask})
	status = Accepted
	task = Task{Cancel: &cancelTrue, Status: &status, UUID: &validUUID}
	expectedResponses = append(expectedResponses, expectedResponse{http.MethodPut, task, http.StatusBadRequest, handlerTask})
	task = Task{Cancel: &cancelTrue, Shell: shell, UUID: &validUUID}
	expectedResponses = append(expectedResponses, expectedResponse{http.MethodPut, task, http.StatusBadRequest, handlerTask})
	task = Task{Cancel: &cancelTrue, UUID: &invalidUUID}
	expectedResponses = append(expectedResponses, expectedResponse{http.MethodPut, task, http.StatusBadRequest, handlerTask})
	task = Task{Cancel: &cancelFalse, UUID: &validUUID}
	expectedResponses = append(expectedResponses, expectedResponse{http.MethodPut, task, http.StatusBadRequest, handlerTask})

	for i := 0; i < len(expectedResponses); i++ {
		testServer := httptest.NewServer(http.HandlerFunc(expectedResponses[i].handlerFunc))
		defer testServer.Close()

		taskBytes, err := json.Marshal(expectedResponses[i].task)
		if err != nil {
			t.Errorf("Marshal error: %v", err)
			return
		}
		client := &http.Client{}
		req, err := http.NewRequest(expectedResponses[i].httpMethod, testServer.URL, bytes.NewBuffer(taskBytes))
		if err != nil {
			t.Errorf("NewRequest error: %v", err)
			return
		}
		resp, err := client.Do(req)
		if err != nil || resp.StatusCode != expectedResponses[i].httpResonse {
			t.Errorf("%s %d with status did not return proper status: %d", expectedResponses[i].httpMethod, i, resp.StatusCode)
			return
		}
	}
}

// TestStatus POSTs several tasks, and validates it can get one or all.
func TestStatus(t *testing.T) {
	testServerStatus := httptest.NewServer(http.HandlerFunc(handlerStatus))
	defer testServerStatus.Close()

	clearTelemetryKVS(t)

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
	getAndUnmarshal(t, testServerStatus.URL, &rtasks)

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
		getAndUnmarshal(t, testServerStatus.URL+query, &rtasks)

		if len(rtasks) != i+1 {
			t.Errorf("status returned incorrect data")
		}
	}
}

// TestTaskDelete does a POST to create a task, updates the status to Completed, then deletes that task.
func TestTaskDelete(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(handlerTask))
	defer testServer.Close()

	task := Task{}
	rtask, err := testTaskPost(t, task)
	if err != nil {
		t.Errorf("could not POST task: %+v", err)
		return
	}
	rtaskBytes, err := json.Marshal(rtask)
	if err != nil {
		t.Errorf("marshal error: %v", err)
		return
	}

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
	req, err := http.NewRequest(http.MethodDelete, testServer.URL, bytes.NewBuffer(rtaskBytes))
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

// TestTaskPost verifies a POST with no expiration gets the default,
// and that a valid POST results in the object correctly being in the KVS.
func TestTaskPost(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(handlerTask))
	defer testServer.Close()

	// Loop0 - POST a task with no expiration and validate it is set to the default.
	// Loop1 - POST a task with an expiration that is not default and validate what is in the KVS.
	for i := 0; i <= 1; i++ {
		var task Task
		// Pick an expiration that is NOT default, and will never be default.
		expOffset := time.Duration(defaultExpirationDuration * 2)
		task = Task{Shell: []string{"ls"}}
		if i == 1 {
			es := time.Now().UTC().Add(expOffset).Format(dateFormat)
			task = Task{Expiration: &es, Shell: []string{"ls"}}
		}

		rtask, err := testTaskPost(t, task)
		if err != nil {
			t.Errorf("could not POST task: %+v", err)
			return
		}

		// Add the UUID, Accepted Status, and Expiration to the sent task and compare to the deserialized task
		task.UUID = rtask.UUID
		acpt := Accepted
		task.Status = &acpt
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
		if !dtask.Equal(&task, time.Second*5) {
			t.Errorf("Sent and deserialized tasks are not equal, task: %+v, dtask: %+v", task, dtask)
			return
		}
	}
}

// TestTaskPut POSTs a new task, validates it status gets changed to Completed, and validates canceling that task
func TestTaskPut(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(handlerTask))
	defer testServer.Close()

	task := Task{}
	rtask, err := testTaskPost(t, task)
	if err != nil {
		t.Errorf("could not POST task: %+v", err)
		return
	}
	tr := true
	rtask.Cancel = &tr
	rtaskBytes, err := json.Marshal(*rtask)
	if err != nil {
		t.Errorf("Marshal error: %v", err)
		return
	}

	// Sleep long enough to make sure the task has been processed by taskRunner
	time.Sleep(taskRunnerCycleTime * 2)
	dtask := Task{}

	// Validate the status is Running
	err = telemetryKVS.Deserialize(rtask.Key(), &dtask)
	if err != nil {
		t.Errorf("Could not deserialize task: %+v", err)
		return
	}
	if dtask.Status == nil || *dtask.Status != Completed {
		t.Errorf("task status is not Completed, is %s", *dtask.Status)
		return
	}

	// Cancel the task
	err = requestAndDeserialize(t, http.MethodPut, testServer.URL, rtaskBytes, rtask.Key(), &dtask)
	if err != nil {
		t.Errorf("could not deserialize task: %+v", err)
		return
	}
	cncl := Canceling
	task.Status = &cncl
	exp := time.Now().Add(-1 * time.Minute).Format(dateFormat)
	// The Equal function requires an expiration...
	dtask.Expiration = &exp
	rtask.Expiration = &exp
	if !dtask.Equal(rtask, 0) {
		t.Errorf("canceled and deserialized tasks are not equal, task: %+v, dtask: %+v", task, dtask)
		return
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
