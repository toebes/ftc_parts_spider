package main

import (
	"testing"
)

// go test -run loadspreadsheet_test
func TestLoadStatusSpreadsheet(t *testing.T) {

	spreadsheetID := "????"
	_, err := LoadStatusSpreadsheet(nil, &spreadsheetID)
	if err != nil {
		t.Log("error should *not* be nil")
	}

	_, err = LoadStatusSpreadsheet(nil, nil)
	if err == nil {
		t.Log("error should be nil")
	}
}
