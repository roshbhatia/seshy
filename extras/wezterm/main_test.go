package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fakeCore(t *testing.T, body string) adapter {
	t.Helper()
	root := t.TempDir()
	core := filepath.Join(root, "core with spaces")
	if err := os.WriteFile(core, []byte("#!/bin/sh\n"+body+"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	return adapter{core: core, timeout: 4 * time.Second}
}
func invoke(t *testing.T, a adapter, capability, id string) response {
	t.Helper()
	r := request{Version: "provider/v1", Kind: "request", RequestID: "test", Capability: capability}
	r.Input.ID = id
	if capability == "picker.create" {
		r.Input.Name = id
	}
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := serve(bytes.NewReader(data), &out, a); err != nil {
		t.Fatal(err)
	}
	var result response
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.RequestID != "test" {
		t.Fatalf("lost request ID: %+v", result)
	}
	return result
}

func TestCreationAdvertisementAndPlan(t *testing.T) {
	a := fakeCore(t, `test "$1" = list || exit 42
printf '[]'`)
	r := invoke(t, a, "picker.create", "review_session")
	if r.Status != "ok" {
		t.Fatal(r)
	}
	data, _ := json.Marshal(r.Output)
	var got plan
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Command) != 4 || got.Command[1] != a.core || got.Command[2] != "--create" || got.Command[3] != "review_session" {
		t.Fatalf("creation command lost arguments: %+v", got)
	}
	if !filepath.IsAbs(got.Cwd) || got.Label != "review_session" {
		t.Fatal(got)
	}
	for _, name := range []string{"", "../outside", "with spaces"} {
		if result := invoke(t, a, "picker.create", name); result.Status != "error" {
			t.Fatalf("accepted %q", name)
		}
	}
	duplicate := fakeCore(t, `printf '[{"name":"review"}]'`)
	if result := invoke(t, duplicate, "picker.create", "review"); result.Status != "error" {
		t.Fatal("accepted duplicate")
	}
	description := invoke(t, a, "picker.describe", "")
	data, _ = json.Marshal(description.Output)
	if !strings.Contains(string(data), `"create"`) {
		t.Fatal("creation prompt missing")
	}
}

func TestInteractiveCreationAndCancellation(t *testing.T) {
	cwd := t.TempDir()
	listing, _ := json.Marshal([]session{{Name: "review", Path: cwd}})
	opening, _ := json.Marshal(map[string]any{"version": "seshy.open/v1", "cwd": cwd, "environment": map[string]string{"SESHY_SESSION": "review"}})
	t.Setenv("PICKER_TEST_LIST", string(listing))
	t.Setenv("PICKER_TEST_OPEN", string(opening))
	a := fakeCore(t, `case "$1" in
new)
  test "$#" = 2 && test "$2" = review || exit 42
  read -r selection
  test "${selection}" = repositories || exit 43
  ;;
list) printf '%s' "${PICKER_TEST_LIST}" ;;
open) printf '%s' "${PICKER_TEST_OPEN}" ;;
esac`)
	created, err := a.createInteractive("review", strings.NewReader("repositories\n"), io.Discard, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if created == nil || created.Cwd != cwd || created.Environment["SESHY_SESSION"] != "review" {
		t.Fatalf("wrong session: %+v", created)
	}
	t.Setenv("PICKER_TEST_LIST", "[]")
	created, err = a.createInteractive("review", strings.NewReader("repositories\n"), io.Discard, io.Discard)
	if err != nil || created != nil {
		t.Fatalf("cancellation opened a session: %+v, %v", created, err)
	}
	failing := fakeCore(t, "exit 23")
	if _, err := failing.createInteractive("review", strings.NewReader(""), io.Discard, io.Discard); err == nil {
		t.Fatal("creation failure was ignored")
	}
}
func TestRuntimeFailures(t *testing.T) {
	for name, body := range map[string]string{"failure": "exit 23", "invalid JSON": "printf garbage", "timeout": "exec sleep 2"} {
		t.Run(name, func(t *testing.T) {
			a := fakeCore(t, body)
			a.timeout = 20 * time.Millisecond
			r := invoke(t, a, "picker.list", "")
			if r.Status != "error" || r.Message == "" {
				t.Fatalf("expected useful error: %+v", r)
			}
		})
	}
	a := adapter{core: filepath.Join(t.TempDir(), "missing"), timeout: time.Second}
	if r := invoke(t, a, "provider.validate", ""); r.Status != "error" {
		t.Fatalf("missing core accepted: %+v", r)
	}
}
func TestInvalidRequest(t *testing.T) {
	for _, input := range []string{"{", `{"version":"bad","kind":"request","requestId":"x"}`} {
		var out bytes.Buffer
		if err := serve(strings.NewReader(input), &out, adapter{}); err != nil {
			t.Fatal(err)
		}
		var r response
		if err := json.Unmarshal(out.Bytes(), &r); err != nil {
			t.Fatal(err)
		}
		if r.Status != "error" {
			t.Fatalf("invalid request accepted: %+v", r)
		}
	}
}
func TestEmptyList(t *testing.T) {
	body := "printf '[]'"
	if defaultCore == "tether" {
		body = "printf '{\"hosts\":[]}'"
	}
	r := invoke(t, fakeCore(t, body), "picker.list", "")
	if r.Status != "ok" {
		t.Fatal(r)
	}
	data, _ := json.Marshal(r.Output)
	if string(data) != `{"items":[]}` {
		t.Fatalf("empty list: %s", data)
	}
}

func TestSessionOpen(t *testing.T) {
	cwd := t.TempDir()
	listing, _ := json.Marshal([]session{{Name: "review with spaces", Path: cwd}})
	p, _ := json.Marshal(map[string]any{"version": "seshy.open/v1", "cwd": cwd, "environment": map[string]string{"SESHY_SESSION": "review with spaces"}})
	a := fakeCore(t, "case \"$1\" in\nlist) printf '%s' '"+string(listing)+"';;\nopen) test \"$4\" = 'review with spaces' || exit 25; printf '%s' '"+string(p)+"';;\nesac")
	r := invoke(t, a, "picker.open", "review with spaces")
	if r.Status != "ok" {
		t.Fatal(r)
	}
	data, _ := json.Marshal(r.Output)
	var got plan
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Cwd != cwd || len(got.Command) != 0 || got.Environment["SESHY_SESSION"] != "review with spaces" {
		t.Fatalf("wrong plan: %+v", got)
	}
	if r := invoke(t, a, "picker.open", "removed"); r.Status != "error" {
		t.Fatal("accepted removed session")
	}
}
func TestSessionRejectsExecutablePlan(t *testing.T) {
	a := fakeCore(t, `case "$1" in
 list) printf '[{"name":"review"}]';;
 open) printf '{"version":"seshy.open/v1","cwd":"/tmp","command":["sh"]}';;
 esac`)
	if r := invoke(t, a, "picker.open", "review"); r.Status != "error" {
		t.Fatal("accepted executable plan")
	}
}
