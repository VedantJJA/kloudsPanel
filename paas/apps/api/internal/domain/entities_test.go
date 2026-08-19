package domain

import "testing"

func TestCanonicalizeEngine(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Postgres", "postgres"},
		{"POSTGRESQL", "postgres"},
		{"postgresql", "postgres"},
		{"pg", "postgres"},
		{"PG", "postgres"},
		{" mysql ", "mysql"},
		{"MySQL", "mysql"},
		{"Mongo", "mongodb"},
		{"mongo", "mongodb"},
		{"mongodb", "mongodb"},
		{"MONGODB", "mongodb"},
		{"redis", "redis"},
		{"REDIS", "redis"},
		{"ClickHouse", "clickhouse"},
		{"clickhouse", "clickhouse"},
		{"", "postgres"},
		{"   ", "postgres"},
		{"cockroachdb", "cockroachdb"},
		{"COCKROACHDB", "cockroachdb"},
	}

	for _, tt := range tests {
		got := CanonicalizeEngine(tt.input)
		if got != tt.expected {
			t.Errorf("CanonicalizeEngine(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}
