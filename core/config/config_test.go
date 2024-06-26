package config

import (
	"os"
	"path/filepath"
	"testing"
)

var (
	dataSourcePath string
)

func init() {
	t := testing.T{}
	testDir := t.TempDir()
	dataSourcePath = filepath.Join(testDir, "test.db")
}

func TestSetGetDelete(t *testing.T) {
	if err := testSetup(); err != nil {
		t.Errorf("testSetup error: %+v", err)
	}

	initializeKVInstance(dataSourcePath)
	value, err := configKVS.Get(configKey)
	if !(value == nil && err == nil) {
		t.Error("Get to empty config should produce nil data and error.")
	}

	vr := "v0.0.0"
	DefaultConfig = Config{Version: &vr}
	err = DefaultConfig.Set()
	if err != nil {
		t.Errorf("error not nil calling Set, error: %v", err)
		return
	}

	rcnfg, err := Get()
	if err != nil {
		t.Errorf("error not nil calling Get, error: %v", err)
		return
	}

	if *rcnfg.Version != vr || rcnfg.HTTPSPort != nil || rcnfg.LogLevel != nil {
		t.Errorf("Get did not produce correct data, rcfng: %+v", rcnfg)
		return
	}

	// Test the merge, which means input config is overriden by saved settings that are non-nil.
	// Version will be overriden to the saved value while HTTPSPort will be as set below.
	vr = "v1.0.0"
	hp := 1
	DefaultConfig = Config{Version: &vr, HTTPSPort: &hp}
	rcnfg, err = Get()
	if err != nil {
		t.Errorf("error not nil calling Get, error: %v", err)
		return
	}
	if *rcnfg.Version != vr || *rcnfg.HTTPSPort != hp || rcnfg.LogLevel != nil {
		t.Errorf("Get did not produce correct data, rcfng: %v", rcnfg)
		return
	}

	if err := Delete(); err != nil {
		t.Errorf("Error calling Delete, err:%v", err)
		return
	}
	value, err = configKVS.Get(configKey)
	if value != nil || err == nil {
		t.Error("Get to empty config produced data or no error")
		return
	}

}

func TestReset(t *testing.T) {
	if err := testSetup(); err != nil {
		t.Errorf("testSetup error: %+v", err)
	}

	// Test dataSourcePath delete
	if _, err := os.Create(dataSourcePath); err != nil {
		t.Errorf("os.Create error:%+v", err)
	}
	if _, err := os.Stat(dataSourcePath); os.IsNotExist(err) {
		t.Error("test delete db was not created")
		return
	}

	// Test the filepathsToDeleteOnReset
	killFileBase := filepath.Join(t.TempDir(), "kill")
	killFile := killFileBase + ".me"
	if _, err := os.Create(killFile); err != nil {
		t.Errorf("os.Create error:%+v", err)
	}
	if _, err := os.Stat(killFile); os.IsNotExist(err) {
		t.Error("test delete file was not created")
		return
	}

	// Test deleting logs.
	lfp := filepath.Join(t.TempDir(), "kill.me.log")
	logFilepath = &lfp
	lfp0 := *logFilepath + ".0"
	lfp1 := *logFilepath + ".1"
	if _, err := os.Create(lfp0); err != nil {
		t.Errorf("os.Create error:%+v", err)
	}
	if _, err := os.Create(lfp1); err != nil {
		t.Errorf("os.Create error:%+v", err)
	}
	if _, err := os.Stat(lfp0); os.IsNotExist(err) {
		t.Error("test log.0 file was not created")
		return
	}
	if _, err := os.Stat(lfp1); os.IsNotExist(err) {
		t.Error("test log.1 file was not created")
		return
	}

	if err := resetIfRequested(true, dataSourcePath, []string{killFileBase + "*"}); err != nil {
		t.Errorf("resetIfRequested error: %v", err)
		return
	}

	if _, err := os.Stat(dataSourcePath); err == nil {
		t.Error("test delete db was not deleted")
		return
	}
	if _, err := os.Stat(killFile); err == nil {
		t.Error("test delete file was not deleted")
		return
	}
	if _, err := os.Stat(lfp0); err == nil {
		t.Error("test delete log.0 was not deleted")
		return
	}
	if _, err := os.Stat(lfp1); err == nil {
		t.Error("test delete log.1 was not deleted")
		return
	}

	// Init("test", dataSourcePath, 1, 1000, 1, 1000, []string{killFile})
}

func testSetup() error {
	return resetIfRequested(true, dataSourcePath, []string{})
}
