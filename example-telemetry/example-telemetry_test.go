package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/paulfdunn/go-helper/logh"
)

type expectedFailedRequest struct {
	httpMethod          string
	task                Task
	expectedHTTPResonse int
	handlerFunc         http.HandlerFunc
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

func TestTaskGet(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(handlerTask))
	defer testServer.Close()

	resp, _ := http.Get(testServer.URL)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("invalid method did not return proper status: %d", resp.StatusCode)
		return
	}
}

// TestExpectedFailureRequests are negative tests to validate API requirements for parameters.
func TestExpectedFailureRequests(t *testing.T) {
	cancelTrue := true
	cancelFalse := false
	command := []string{"cmd"}
	exp := time.Now().Add(time.Minute).Format(dateFormat)
	status := Accepted
	shell := []string{"shell"}
	taskInvalidUUID := uuid.New()
	taskValidUUID := uuid.New()
	task := Task{UUID: &taskValidUUID}
	telemetryKVS.Serialize(task.UUID.String(), task)

	expectedFailedRequests := []expectedFailedRequest{}

	// http.MethoDelete tests
	task = Task{}
	expectedFailedRequests = append(expectedFailedRequests, expectedFailedRequest{http.MethodDelete, task, http.StatusBadRequest, handlerTask})
	task = Task{UUID: &taskInvalidUUID}
	expectedFailedRequests = append(expectedFailedRequests, expectedFailedRequest{http.MethodDelete, task, http.StatusBadRequest, handlerTask})
	task = Task{Cancel: &cancelTrue, UUID: &taskValidUUID}
	expectedFailedRequests = append(expectedFailedRequests, expectedFailedRequest{http.MethodDelete, task, http.StatusBadRequest, handlerTask})
	task = Task{Command: command, UUID: &taskValidUUID}
	expectedFailedRequests = append(expectedFailedRequests, expectedFailedRequest{http.MethodDelete, task, http.StatusBadRequest, handlerTask})
	// Make sure the time is in the future.
	task = Task{Expiration: &exp, UUID: &taskValidUUID}
	expectedFailedRequests = append(expectedFailedRequests, expectedFailedRequest{http.MethodDelete, task, http.StatusBadRequest, handlerTask})
	task = Task{Status: &status, UUID: &taskValidUUID}
	expectedFailedRequests = append(expectedFailedRequests, expectedFailedRequest{http.MethodDelete, task, http.StatusBadRequest, handlerTask})
	task = Task{Shell: shell, UUID: &taskValidUUID}
	expectedFailedRequests = append(expectedFailedRequests, expectedFailedRequest{http.MethodDelete, task, http.StatusBadRequest, handlerTask})

	// http.MethodPost tests
	// Post with a UUID is not valid
	task = Task{UUID: &taskValidUUID}
	expectedFailedRequests = append(expectedFailedRequests, expectedFailedRequest{http.MethodPost, task, http.StatusBadRequest, handlerTask})
	// POST with a Status (loop 0) or Cancel (loop1) are not valid
	task = Task{Status: &status}
	expectedFailedRequests = append(expectedFailedRequests, expectedFailedRequest{http.MethodPost, task, http.StatusBadRequest, handlerTask})
	task = Task{Cancel: &cancelTrue}
	expectedFailedRequests = append(expectedFailedRequests, expectedFailedRequest{http.MethodPost, task, http.StatusBadRequest, handlerTask})
	// POST with a expiration in the past is not valid.
	task = Task{Expiration: &exp}
	expectedFailedRequests = append(expectedFailedRequests, expectedFailedRequest{http.MethodPost, task, http.StatusBadRequest, handlerTask})

	// http.MethodPut tests
	// PUT without both Cancel and UUID, with an invalid UUID, or with a Command, Expiration,
	// Shell, or Status are not valid.
	task = Task{Cancel: &cancelTrue}
	expectedFailedRequests = append(expectedFailedRequests, expectedFailedRequest{http.MethodPut, task, http.StatusBadRequest, handlerTask})
	task = Task{UUID: &taskValidUUID}
	expectedFailedRequests = append(expectedFailedRequests, expectedFailedRequest{http.MethodPut, task, http.StatusBadRequest, handlerTask})
	task = Task{Cancel: &cancelTrue, Command: command, UUID: &taskValidUUID}
	expectedFailedRequests = append(expectedFailedRequests, expectedFailedRequest{http.MethodPut, task, http.StatusBadRequest, handlerTask})
	// Make sure the time is in the future.
	exp = time.Now().Add(time.Minute).Format(dateFormat)
	task = Task{Cancel: &cancelTrue, Expiration: &exp, UUID: &taskValidUUID}
	expectedFailedRequests = append(expectedFailedRequests, expectedFailedRequest{http.MethodPut, task, http.StatusBadRequest, handlerTask})
	status = Accepted
	task = Task{Cancel: &cancelTrue, Status: &status, UUID: &taskValidUUID}
	expectedFailedRequests = append(expectedFailedRequests, expectedFailedRequest{http.MethodPut, task, http.StatusBadRequest, handlerTask})
	task = Task{Cancel: &cancelTrue, Shell: shell, UUID: &taskValidUUID}
	expectedFailedRequests = append(expectedFailedRequests, expectedFailedRequest{http.MethodPut, task, http.StatusBadRequest, handlerTask})
	task = Task{Cancel: &cancelTrue, UUID: &taskInvalidUUID}
	expectedFailedRequests = append(expectedFailedRequests, expectedFailedRequest{http.MethodPut, task, http.StatusBadRequest, handlerTask})
	task = Task{Cancel: &cancelFalse, UUID: &taskValidUUID}
	expectedFailedRequests = append(expectedFailedRequests, expectedFailedRequest{http.MethodPut, task, http.StatusBadRequest, handlerTask})

	for i := 0; i < len(expectedFailedRequests); i++ {
		testServer := httptest.NewServer(http.HandlerFunc(expectedFailedRequests[i].handlerFunc))
		defer testServer.Close()

		taskBytes, err := json.Marshal(expectedFailedRequests[i].task)
		if err != nil {
			t.Errorf("Marshal error: %v", err)
			return
		}
		client := &http.Client{}
		req, err := http.NewRequest(expectedFailedRequests[i].httpMethod, testServer.URL, bytes.NewBuffer(taskBytes))
		if err != nil {
			t.Errorf("NewRequest error: %v", err)
			return
		}
		resp, err := client.Do(req)
		if err != nil || resp.StatusCode != expectedFailedRequests[i].expectedHTTPResonse {
			t.Errorf("%s %d with status did not return proper status: %d", expectedFailedRequests[i].httpMethod, i, resp.StatusCode)
			return
		}
	}
}

// TestTaskDelete does a POST to create a task, then deletes that task.
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
		err = telemetryKVS.Deserialize(rtask.UUID.String(), &dtask)
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

// TestTaskPut validates canceling a task.
func TestTaskPut(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(handlerTask))
	defer testServer.Close()

	// Store a task, then cancel it and validate the KVS is updated.
	nu := uuid.New()
	task := Task{UUID: &nu}
	telemetryKVS.Serialize(task.UUID.String(), task)
	tr := true
	task.Cancel = &tr
	taskBytes, err := json.Marshal(task)
	if err != nil {
		t.Errorf("Marshal error: %v", err)
		return
	}
	client := &http.Client{}
	req, err := http.NewRequest(http.MethodPut, testServer.URL, bytes.NewBuffer(taskBytes))
	if err != nil {
		t.Errorf("NewRequest error: %v", err)
		return
	}
	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != http.StatusAccepted {
		t.Errorf("PUT did not return proper status: %d", resp.StatusCode)
		return
	}
	// validate
	dtask := Task{}
	err = telemetryKVS.Deserialize(task.UUID.String(), &dtask)
	if err != nil {
		t.Errorf("could not deserialize task: %+v", err)
		return
	}
	cncl := Canceling
	task.Status = &cncl
	exp := time.Now().Add(-1 * time.Minute).Format(dateFormat)
	// The Equal function requires an expiration...
	dtask.Expiration = &exp
	task.Expiration = &exp
	if !dtask.Equal(&task, 0) {
		t.Errorf("canceled and deserialized tasks are not equal, task: %+v, dtask: %+v", task, dtask)
		return
	}
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
	resp, _ := http.Post(testServer.URL, "application/json", bytes.NewBuffer(taskBytes))
	if resp.StatusCode != http.StatusOK {
		t.Errorf("invalid method did not return proper status: %d", resp.StatusCode)
		return nil, err
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

	// lp = logh.Map[config.LogName].Println
	lpf = logh.Map[""].Printf

	tempDir := t.TempDir()
	fmt.Printf("tempDir: %s\n", tempDir)
	runtimeConfig.PersistentDirectory = &tempDir

	// Every call to TempDir returns a unique directory; there is no need to remove files.
	initializeKVS(tempDir, appName+telemetryFileSuffix)
}
