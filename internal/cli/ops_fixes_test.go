package cli

import "testing"

func TestWantsRemoteDeploy(t *testing.T) {
	if wantsRemoteDeploy([]string{"--ping", "--app", "todo"}) {
		t.Fatal("workspace --ping must not be treated as remote deploy")
	}
	if !wantsRemoteDeploy([]string{"--remote", "user@host"}) {
		t.Fatal("expected remote")
	}
	if !wantsRemoteDeploy([]string{"user@host"}) {
		t.Fatal("user@host is remote")
	}
}

func TestParseDeployLocalPing(t *testing.T) {
	o, err := parseDeployLocalFlags([]string{"--ping", "--app", "todo"})
	if err != nil {
		t.Fatal(err)
	}
	if !o.ping || len(o.apps) != 1 || o.apps[0] != "todo" {
		t.Fatalf("%+v", o)
	}
}

func TestParseLeaseArgs(t *testing.T) {
	dir, err := parseLeaseArgs(nil)
	if err != nil || dir != "." {
		t.Fatalf("bare lease: dir=%q err=%v", dir, err)
	}
	dir, err = parseLeaseArgs([]string{"watch", "--dir", "apps/todo"})
	if err != nil || dir != "apps/todo" {
		t.Fatalf("watch: dir=%q err=%v", dir, err)
	}
	dir, err = parseLeaseArgs([]string{"apps/admin"})
	if err != nil || dir != "apps/admin" {
		t.Fatalf("dir arg: dir=%q err=%v", dir, err)
	}
}
