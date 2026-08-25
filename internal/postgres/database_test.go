package postgres

import (
	"database/sql"
	"strings"
	"testing"
)

func TestGORMArgumentsUseStableNames(t *testing.T) {
	arguments := gormArguments("first", "second")
	if len(arguments) != 2 {
		t.Fatalf("arguments = %#v", arguments)
	}
	for index, expected := range []struct {
		name  string
		value string
	}{{name: "p1", value: "first"}, {name: "p2", value: "second"}} {
		argument, ok := arguments[index].(sql.NamedArg)
		if !ok || argument.Name != expected.name || argument.Value != expected.value {
			t.Fatalf("argument %d = %#v", index, arguments[index])
		}
	}
}

func TestValidateDestructiveTestDatabaseURL(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name        string
		databaseURL string
		wantError   string
	}{
		{name: "dedicated test database", databaseURL: "postgres://user:password@localhost:5432/aegislink_test?sslmode=disable"},
		{name: "development database", databaseURL: "postgres://user:password@localhost:5432/aegislink?sslmode=disable", wantError: "dedicated *_test database"},
		{name: "missing database", databaseURL: "postgres://user:password@localhost:5432", wantError: "dedicated *_test database"},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := validateDestructiveTestDatabaseURL(test.databaseURL)
			if test.wantError == "" && err != nil {
				t.Fatal(err)
			}
			if test.wantError != "" && (err == nil || !strings.Contains(err.Error(), test.wantError)) {
				t.Fatalf("error = %v, want substring %q", err, test.wantError)
			}
		})
	}
}
