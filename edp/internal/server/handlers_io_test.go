package server

import "testing"

func TestParseImportAcceptsCredentials(t *testing.T) {
	// a single env object carrying credentials is imported verbatim
	items, err := parseImport([]byte(`{"name":"x","source_type":"git","git_token":"GIT-T","registry_password":"REG-P"}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("got %d items", len(items))
	}
	if items[0].GitToken != "GIT-T" || items[0].RegistryPassword != "REG-P" {
		t.Errorf("credentials not imported: token=%q pw=%q", items[0].GitToken, items[0].RegistryPassword)
	}
}

func TestParseImportShapes(t *testing.T) {
	bundle := `{"version":1,"environments":[{"name":"a"},{"name":"b","timed_hooks":[{"name":"h"}]}]}`
	items, err := parseImport([]byte(bundle))
	if err != nil || len(items) != 2 {
		t.Fatalf("bundle: items=%d err=%v", len(items), err)
	}
	if len(items[1].TimedHooks) != 1 || items[1].TimedHooks[0].Name != "h" {
		t.Errorf("nested timed hook not parsed: %+v", items[1].TimedHooks)
	}

	arr, err := parseImport([]byte(`[{"name":"a"},{"name":"b"}]`))
	if err != nil || len(arr) != 2 {
		t.Fatalf("array: items=%d err=%v", len(arr), err)
	}

	if _, err := parseImport([]byte(`{"version":1,"environments":[]}`)); err == nil {
		t.Error("expected error for empty/no environments")
	}
	if _, err := parseImport([]byte(`not json`)); err == nil {
		t.Error("expected error for invalid JSON")
	}
}
