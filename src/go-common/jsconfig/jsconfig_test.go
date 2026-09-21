package jsconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/riclolsen/json-scada/src/go-common/jslog"
)

func writeTemp(t *testing.T, name, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

const goodJSON = `{
  "nodeName": "  mainNode  ",
  "mongoConnectionString": " mongodb://localhost:27017/?replicaSet=rs1 ",
  "mongoDatabaseName": " json_scada ",
  "tlsCaPemFile": "/etc/ca.pem"
}`

func TestLoadTrimsTheThreeStringFields(t *testing.T) {
	cfg, err := Load(writeTemp(t, "c.json", goodJSON))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.NodeName != "mainNode" {
		t.Errorf("NodeName = %q, want trimmed", cfg.NodeName)
	}
	if cfg.MongoDatabaseName != "json_scada" {
		t.Errorf("MongoDatabaseName = %q, want trimmed", cfg.MongoDatabaseName)
	}
	if cfg.MongoConnectionString != "mongodb://localhost:27017/?replicaSet=rs1" {
		t.Errorf("MongoConnectionString = %q, want trimmed", cfg.MongoConnectionString)
	}
	if cfg.TLSCaPemFile != "/etc/ca.pem" {
		t.Errorf("TLSCaPemFile = %q", cfg.TLSCaPemFile)
	}
}

func TestValidateReportsMissingFieldsInDriverOrder(t *testing.T) {
	if err := Validate(Config{}, "c.json"); err == nil ||
		!strings.Contains(err.Error(), "connection string") {
		t.Errorf("empty config: want connection-string error, got %v", err)
	}
	if err := Validate(Config{MongoConnectionString: "x"}, "c.json"); err == nil ||
		!strings.Contains(err.Error(), "database name") {
		t.Errorf("want database-name error, got %v", err)
	}
	if err := Validate(Config{MongoConnectionString: "x", MongoDatabaseName: "y"}, "c.json"); err == nil ||
		!strings.Contains(err.Error(), "nodeName") {
		t.Errorf("want nodeName error, got %v", err)
	}
	full := Config{MongoConnectionString: "x", MongoDatabaseName: "y", NodeName: "z"}
	if err := Validate(full, "c.json"); err != nil {
		t.Errorf("complete config should validate, got %v", err)
	}
}

func TestParseArgs(t *testing.T) {
	defer jslog.SetLevel(jslog.LevelBasic)

	a := ParseArgs([]string{"drv"})
	if a.InstanceNumber != 1 || a.LogLevelFromCLI || a.ConfigFilePath != "" {
		t.Errorf("no args: %+v", a)
	}

	a = ParseArgs([]string{"drv", " 3 ", "2", "/tmp/x.json"})
	if a.InstanceNumber != 3 {
		t.Errorf("InstanceNumber = %d, want 3 (whitespace tolerated)", a.InstanceNumber)
	}
	if !a.LogLevelFromCLI || jslog.Level() != 2 {
		t.Errorf("log level not applied: fromCLI=%v level=%d", a.LogLevelFromCLI, jslog.Level())
	}
	if a.ConfigFilePath != "/tmp/x.json" {
		t.Errorf("ConfigFilePath = %q", a.ConfigFilePath)
	}
}

// parity: a malformed number is logged and ignored, never fatal.
func TestParseArgsIgnoresMalformedNumbers(t *testing.T) {
	defer jslog.SetLevel(jslog.LevelBasic)
	a := ParseArgs([]string{"drv", "notanumber", "alsonot"})
	if a.InstanceNumber != 1 {
		t.Errorf("InstanceNumber = %d, want the default 1", a.InstanceNumber)
	}
	if a.LogLevelFromCLI {
		t.Error("a malformed log level must not count as given on the CLI")
	}
}

func TestResolvePathPrecedence(t *testing.T) {
	dir := t.TempDir()
	mk := func(n string) string {
		p := filepath.Join(dir, n)
		if err := os.WriteFile(p, []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	primary, fallback, arg, env := mk("primary"), mk("fallback"), mk("arg"), mk("env")

	t.Setenv(ConfigFileEnvVar, "")
	if got := ResolvePath(primary, fallback, arg); got != arg {
		t.Errorf("argv[3] should win: got %q", got)
	}

	t.Setenv(ConfigFileEnvVar, env)
	if got := ResolvePath(primary, fallback, ""); got != env {
		t.Errorf("env should beat primary: got %q", got)
	}
	if got := ResolvePath(primary, fallback, arg); got != arg {
		t.Errorf("argv[3] should beat env: got %q", got)
	}

	// An env var naming a file that does not exist is ignored, not honoured
	// and then failed on later.
	t.Setenv(ConfigFileEnvVar, filepath.Join(dir, "nope"))
	if got := ResolvePath(primary, fallback, ""); got != primary {
		t.Errorf("unreadable env should be ignored: got %q", got)
	}

	t.Setenv(ConfigFileEnvVar, "")
	if got := ResolvePath(filepath.Join(dir, "nope"), fallback, ""); got != fallback {
		t.Errorf("missing primary should fall back: got %q", got)
	}
}

func TestExpandHome(t *testing.T) {
	t.Setenv("HOME", "/home/u")
	if got := ExpandHome("~/a/b.json"); got != "/home/u/a/b.json" {
		t.Errorf("ExpandHome = %q", got)
	}
	if got := ExpandHome("/abs/p"); got != "/abs/p" {
		t.Errorf("absolute path must not change: %q", got)
	}

	// USERPROFILE is the Windows fallback.
	t.Setenv("HOME", "")
	t.Setenv("USERPROFILE", `C:\Users\u`)
	if got := ExpandHome("~/a"); got != `C:\Users\u/a` {
		t.Errorf("USERPROFILE fallback = %q", got)
	}
}

func TestLoadReportsMissingFile(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "absent.json")); err == nil {
		t.Error("want an error for a missing file")
	}
}

func TestLoadReportsBadJSON(t *testing.T) {
	if _, err := Load(writeTemp(t, "bad.json", "{not json")); err == nil {
		t.Error("want an error for malformed JSON")
	}
}
