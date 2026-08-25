package postgres

import (
	"reflect"
	"testing"
)

func TestPostgresStatementRebindsNumberedParameters(t *testing.T) {
	statement, variables := postgresStatement(
		"SELECT $2, $1, $2::text, '$3 is literal text'",
		"first",
		"second",
		"unused",
	)
	if statement != "SELECT ?, ?, ?::text, '$3 is literal text'" {
		t.Fatalf("statement = %q", statement)
	}
	if want := []any{"second", "first", "second"}; !reflect.DeepEqual(variables, want) {
		t.Fatalf("variables = %#v, want %#v", variables, want)
	}
}
