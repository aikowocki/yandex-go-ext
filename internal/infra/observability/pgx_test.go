package observability

import "testing"

func TestSQLOperationSkipsLeadingComments(t *testing.T) {
	tests := []struct {
		query string
		want  string
	}{
		{query: "SELECT 1", want: "SELECT"},
		{query: "-- name: GetAvatar :one\nSELECT * FROM avatars", want: "SELECT"},
		{query: "/* migration */\nINSERT INTO avatars VALUES (1)", want: "INSERT"},
		{query: "-- comment only", want: "QUERY"},
		{query: "", want: "QUERY"},
	}
	for _, test := range tests {
		if got := sqlOperation(test.query); got != test.want {
			t.Errorf("sqlOperation(%q) = %q, want %q", test.query, got, test.want)
		}
	}
}
