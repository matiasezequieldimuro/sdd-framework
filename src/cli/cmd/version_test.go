package cmd

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestVersionCommandJSONEnvelope(t *testing.T) {
	root := NewRootCommand(Application{})
	var stdout bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"--json", "version"})

	if err := executeRoot(root); err != nil {
		t.Fatalf("executeRoot() error = %v", err)
	}
	var response JSONResponse
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		t.Fatalf("Unmarshal() error = %v; output = %s", err, stdout.String())
	}
	if !response.Success || response.Error != nil {
		t.Fatalf("response = %#v", response)
	}
	data, ok := response.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("data = %#v", response.Data)
	}
	if _, ok := data["version"]; !ok {
		t.Fatalf("missing version field: %#v", data)
	}
}

func TestVersionCommandText(t *testing.T) {
	root := NewRootCommand(Application{})
	var stdout bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"version"})

	if err := executeRoot(root); err != nil {
		t.Fatalf("executeRoot() error = %v", err)
	}
	if !bytes.Contains(stdout.Bytes(), []byte("sdd-cli")) {
		t.Fatalf("stdout = %q", stdout.String())
	}
}
