package main

import (
	"errors"
	"flag"
	"fmt"
	"strings"
)

// envPrefix is the prefix of the environment variables that configure filebin
const envPrefix = "FILEBIN_"

// envIgnored lists flags that cannot be set from the environment
var envIgnored = map[string]bool{
	"version": true,
}

// envName returns the environment variable that configures the flag with the
// given name: the flag name in upper case with underscores instead of hyphens
// and the FILEBIN_ prefix. The database flags use DATABASE instead of DB.
func envName(name string) string {
	if rest, ok := strings.CutPrefix(name, "db-"); ok {
		name = "database-" + rest
	}
	return envPrefix + strings.ToUpper(strings.ReplaceAll(name, "-", "_"))
}

// applyEnv sets every flag in fs that was not given on the command line from
// its environment variable, if that variable is set and not empty. Command
// line flags take precedence over environment variables. Values are parsed by
// the flag itself, so an environment variable accepts the same values as the
// flag, and boolean flags also accept yes and no. Values that cannot be parsed
// are returned as errors rather than ignored.
func applyEnv(fs *flag.FlagSet, getenv func(string) string) error {
	given := make(map[string]bool)
	fs.Visit(func(f *flag.Flag) {
		given[f.Name] = true
	})

	var errs []error
	fs.VisitAll(func(f *flag.Flag) {
		if given[f.Name] || envIgnored[f.Name] {
			return
		}
		name := envName(f.Name)
		value := getenv(name)
		if value == "" {
			return
		}
		if isBoolFlag(f) {
			switch strings.ToLower(value) {
			case "yes":
				value = "true"
			case "no":
				value = "false"
			}
		}
		previous := f.Value.String()
		if err := fs.Set(f.Name, value); err != nil {
			// Some flag types, like bool, change their value even when
			// parsing fails
			_ = f.Value.Set(previous)
			errs = append(errs, fmt.Errorf("%s=%q: %w", name, value, err))
		}
	})
	return errors.Join(errs...)
}

// isBoolFlag reports whether f is a boolean flag
func isBoolFlag(f *flag.Flag) bool {
	b, ok := f.Value.(interface{ IsBoolFlag() bool })
	return ok && b.IsBoolFlag()
}
