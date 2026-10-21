package appcfg

import "testing"

func TestParseTodo(t *testing.T) {
	a, err := Parse(`
# comment
name = "todo"
deploy_mode = "push"
kind = "split"
zone = "hyperreader.eu"
page_subdomain = "todo"
api_subdomain = "todo-api"
port = 18787
attachments = true
tunnel_name = "cogs-todo-app"
`)
	if err != nil {
		t.Fatal(err)
	}
	if a.Port != 18787 || a.Zone != "hyperreader.eu" || !a.Attachments {
		t.Fatalf("%+v", a)
	}
	if got := a.APIHost(); got != "todo-api.hyperreader.eu" {
		t.Fatal(got)
	}
	if a.KindManifest() != "" || a.TunnelName != "cogs-todo-app" {
		t.Fatalf("%+v", a)
	}
}

func TestParseCombined(t *testing.T) {
	a, err := Parse("name = \"admin\"\nkind = \"combined\"\nzone = \"hyperreader.eu\"\npage_subdomain = \"cogs-admin\"\n")
	if err != nil {
		t.Fatal(err)
	}
	if a.APIHost() != "cogs-admin.hyperreader.eu" {
		t.Fatal(a.APIHost())
	}
}
