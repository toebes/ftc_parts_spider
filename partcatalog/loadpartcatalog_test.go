package partcatalog

import (
	"testing"
)

// go test -run loadspreadsheet_test
func TestLoadPartCatalog(t *testing.T) {

	spreadsheetID := "????"
	_, err := LoadPartCatalog(&spreadsheetID, nil)
	if err != nil {
		t.Log("error should *not* be nil")
	}

	_, err = LoadPartCatalog(nil, nil)
	if err == nil {
		t.Log("error should be nil")
	}
}
