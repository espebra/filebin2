package main

import (
	"flag"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

func TestEnvName(t *testing.T) {
	tests := map[string]string{
		"manual-approval":             "FILEBIN_MANUAL_APPROVAL",
		"require-verification-cookie": "FILEBIN_REQUIRE_VERIFICATION_COOKIE",
		"s3-secret-key":               "FILEBIN_S3_SECRET_KEY",
		"db-host":                     "FILEBIN_DATABASE_HOST",
		"db-conn-max-idle-time":       "FILEBIN_DATABASE_CONN_MAX_IDLE_TIME",
		"contact":                     "FILEBIN_CONTACT",
	}
	for flagName, want := range tests {
		if got := envName(flagName); got != want {
			t.Errorf("envName(%q) = %q, want %q", flagName, got, want)
		}
	}
}

// testFlags is a set of flags of each type, parsed from args
type testFlags struct {
	fs         *flag.FlagSet
	approval   *bool
	secure     *bool
	expiration *int
	timeout    *time.Duration
	contact    *string
	dbHost     *string
	version    *bool
}

func newTestFlags(t *testing.T, args ...string) testFlags {
	t.Helper()
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	f := testFlags{
		fs:         fs,
		approval:   fs.Bool("manual-approval", false, ""),
		secure:     fs.Bool("s3-secure", true, ""),
		expiration: fs.Int("expiration", 604800, ""),
		timeout:    fs.Duration("read-timeout", time.Hour, ""),
		contact:    fs.String("contact", "", ""),
		dbHost:     fs.String("db-host", "", ""),
		version:    fs.Bool("version", false, ""),
	}
	if err := fs.Parse(args); err != nil {
		t.Fatal(err)
	}
	return f
}

func envFrom(env map[string]string) func(string) string {
	return func(name string) string { return env[name] }
}

func TestApplyEnvBool(t *testing.T) {
	tests := []struct {
		value string
		want  bool
	}{
		{"true", true},
		{"True", true},
		{"TRUE", true},
		{"1", true},
		{"t", true},
		{"yes", true},
		{"Yes", true},
		{"false", false},
		{"False", false},
		{"0", false},
		{"no", false},
		{"NO", false},
	}
	for _, tc := range tests {
		f := newTestFlags(t)
		if err := applyEnv(f.fs, envFrom(map[string]string{"FILEBIN_MANUAL_APPROVAL": tc.value})); err != nil {
			t.Errorf("FILEBIN_MANUAL_APPROVAL=%q: unexpected error: %v", tc.value, err)
			continue
		}
		if *f.approval != tc.want {
			t.Errorf("FILEBIN_MANUAL_APPROVAL=%q: got %t, want %t", tc.value, *f.approval, tc.want)
		}
	}
}

func TestApplyEnvInvalidValues(t *testing.T) {
	tests := map[string]string{
		"FILEBIN_MANUAL_APPROVAL": "enabled",
		"FILEBIN_S3_SECURE":       "on",
		"FILEBIN_EXPIRATION":      "a week",
		"FILEBIN_READ_TIMEOUT":    "60",
	}
	for name, value := range tests {
		f := newTestFlags(t)
		err := applyEnv(f.fs, envFrom(map[string]string{name: value}))
		if err == nil {
			t.Errorf("%s=%q: expected an error", name, value)
			continue
		}
		if !strings.Contains(err.Error(), name) {
			t.Errorf("%s=%q: expected the error to name the variable, got: %v", name, value, err)
		}
	}

	// An invalid value leaves the default in place
	f := newTestFlags(t)
	_ = applyEnv(f.fs, envFrom(map[string]string{"FILEBIN_MANUAL_APPROVAL": "enabled", "FILEBIN_S3_SECURE": "on"}))
	if *f.approval != false || *f.secure != true {
		t.Errorf("expected invalid values to leave the defaults, got manual-approval=%t s3-secure=%t", *f.approval, *f.secure)
	}
}

func TestApplyEnvReportsAllErrors(t *testing.T) {
	f := newTestFlags(t)
	err := applyEnv(f.fs, envFrom(map[string]string{
		"FILEBIN_MANUAL_APPROVAL": "enabled",
		"FILEBIN_EXPIRATION":      "a week",
	}))
	if err == nil {
		t.Fatal("expected an error")
	}
	for _, name := range []string{"FILEBIN_MANUAL_APPROVAL", "FILEBIN_EXPIRATION"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("expected the error to name %s, got: %v", name, err)
		}
	}
}

func TestApplyEnvFlagsTakePrecedence(t *testing.T) {
	env := envFrom(map[string]string{
		"FILEBIN_MANUAL_APPROVAL": "false",
		"FILEBIN_S3_SECURE":       "true",
		"FILEBIN_EXPIRATION":      "60",
		"FILEBIN_READ_TIMEOUT":    "1m",
		"FILEBIN_CONTACT":         "env@example.com",
		"FILEBIN_DATABASE_HOST":   "env-db",
	})

	// Flags given on the command line win, including when they are given
	// their default value
	f := newTestFlags(t,
		"--manual-approval",
		"--s3-secure=false",
		"--expiration=604800",
		"--read-timeout=1h",
		"--contact=flag@example.com",
		"--db-host=flag-db",
	)
	if err := applyEnv(f.fs, env); err != nil {
		t.Fatal(err)
	}
	if !*f.approval {
		t.Error("expected --manual-approval to win over FILEBIN_MANUAL_APPROVAL=false")
	}
	if *f.secure {
		t.Error("expected --s3-secure=false to win over FILEBIN_S3_SECURE=true")
	}
	if *f.expiration != 604800 {
		t.Errorf("expected --expiration=604800 to win over FILEBIN_EXPIRATION, got %d", *f.expiration)
	}
	if *f.timeout != time.Hour {
		t.Errorf("expected --read-timeout=1h to win over FILEBIN_READ_TIMEOUT, got %s", *f.timeout)
	}
	if *f.contact != "flag@example.com" {
		t.Errorf("expected --contact to win over FILEBIN_CONTACT, got %q", *f.contact)
	}
	if *f.dbHost != "flag-db" {
		t.Errorf("expected --db-host to win over FILEBIN_DATABASE_HOST, got %q", *f.dbHost)
	}
}

func TestApplyEnvSetsFlagsNotGiven(t *testing.T) {
	f := newTestFlags(t)
	err := applyEnv(f.fs, envFrom(map[string]string{
		"FILEBIN_MANUAL_APPROVAL": "true",
		"FILEBIN_S3_SECURE":       "false",
		"FILEBIN_EXPIRATION":      "60",
		"FILEBIN_READ_TIMEOUT":    "1m",
		"FILEBIN_CONTACT":         "env@example.com",
		"FILEBIN_DATABASE_HOST":   "env-db",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !*f.approval || *f.secure || *f.expiration != 60 || *f.timeout != time.Minute || *f.contact != "env@example.com" || *f.dbHost != "env-db" {
		t.Errorf("expected all flags to be set from the environment, got manual-approval=%t s3-secure=%t expiration=%d read-timeout=%s contact=%q db-host=%q",
			*f.approval, *f.secure, *f.expiration, *f.timeout, *f.contact, *f.dbHost)
	}
}

func TestApplyEnvEmptyAndUnset(t *testing.T) {
	f := newTestFlags(t)
	if err := applyEnv(f.fs, envFrom(map[string]string{"FILEBIN_S3_SECURE": "", "FILEBIN_DB_HOST": "wrong-name"})); err != nil {
		t.Fatal(err)
	}
	if !*f.secure {
		t.Error("expected an empty variable to leave the default")
	}
	if *f.dbHost != "" {
		t.Errorf("expected FILEBIN_DB_HOST to be ignored, got %q", *f.dbHost)
	}
}

func TestApplyEnvIgnoresVersion(t *testing.T) {
	f := newTestFlags(t)
	if err := applyEnv(f.fs, envFrom(map[string]string{"FILEBIN_VERSION": "true"})); err != nil {
		t.Fatal(err)
	}
	if *f.version {
		t.Error("expected FILEBIN_VERSION to be ignored")
	}
}

// Every flag of the binary maps to an environment variable that is documented
// in the README
func TestEnvNamesDocumented(t *testing.T) {
	data, err := os.ReadFile("../../README.md")
	if err != nil {
		t.Fatal(err)
	}
	readme := string(data)
	flag.CommandLine.VisitAll(func(f *flag.Flag) {
		// Skip the flags registered by the testing package
		if envIgnored[f.Name] || strings.HasPrefix(f.Name, "test.") {
			return
		}
		name := envName(f.Name)
		if !strings.Contains(readme, "`"+name+"`") {
			t.Errorf("--%s: %s is not documented in README.md", f.Name, name)
		}
	})
}
