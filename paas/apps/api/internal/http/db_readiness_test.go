package http

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yourorg/klouds/api/internal/domain"
)

// stubCommandRunner simulates container execution responses for unit testing.
type stubCommandRunner struct {
	failCount int32
	callCount int32
}

func (s *stubCommandRunner) Run(ctx context.Context, name string, args ...string) error {
	calls := atomic.AddInt32(&s.callCount, 1)
	if calls <= atomic.LoadInt32(&s.failCount) {
		return errors.New("container not accepting connections yet")
	}
	return nil
}

func TestWaitForDatabaseReady_Success(t *testing.T) {
	db := &domain.Database{
		ID:               "db-1",
		Name:             "test-postgres",
		Engine:           "postgres",
		InternalHostname: "paas-db-test-postgres",
		ResourceJSON:     `{"username":"postgres","password":"secretpassword","databaseName":"testdb"}`,
	}

	// Succeeds on 3rd attempt
	stub := &stubCommandRunner{failCount: 2}
	ctx := context.Background()

	start := time.Now()
	ready := waitForDatabaseReadyWithRunner(ctx, stub, db, 2*time.Second, 20*time.Millisecond)
	elapsed := time.Since(start)

	if !ready {
		t.Fatalf("expected database to become ready, got false")
	}
	if stub.callCount < 3 {
		t.Fatalf("expected at least 3 probe calls, got %d", stub.callCount)
	}
	if elapsed > 1*time.Second {
		t.Fatalf("expected probe to finish promptly, took %v", elapsed)
	}
}

func TestWaitForDatabaseReady_Timeout(t *testing.T) {
	db := &domain.Database{
		ID:               "db-2",
		Name:             "test-mysql",
		Engine:           "mysql",
		InternalHostname: "paas-db-test-mysql",
		ResourceJSON:     `{"username":"root","password":"secretpassword","databaseName":"testdb"}`,
	}

	// Always fails
	stub := &stubCommandRunner{failCount: 999}
	ctx := context.Background()

	start := time.Now()
	ready := waitForDatabaseReadyWithRunner(ctx, stub, db, 150*time.Millisecond, 25*time.Millisecond)
	elapsed := time.Since(start)

	if ready {
		t.Fatalf("expected probe to fail on timeout, got true")
	}
	if elapsed < 100*time.Millisecond {
		t.Fatalf("probe returned too early before timeout, took %v", elapsed)
	}
}

func TestBuildReadinessProbeArgs(t *testing.T) {
	tests := []struct {
		name     string
		db       *domain.Database
		expected []string
	}{
		{
			name: "PostgreSQL",
			db: &domain.Database{
				Name:             "mydb",
				Engine:           "postgres",
				InternalHostname: "paas-db-mydb",
				ResourceJSON:     `{"username":"custom_user"}`,
			},
			expected: []string{"exec", "paas-db-mydb", "pg_isready", "-U", "custom_user"},
		},
		{
			name: "Redis",
			db: &domain.Database{
				Name:             "myredis",
				Engine:           "redis",
				InternalHostname: "paas-db-myredis",
				ResourceJSON:     `{"password":"redissecret"}`,
			},
			expected: []string{"exec", "paas-db-myredis", "redis-cli", "-a", "redissecret", "ping"},
		},
		{
			name: "MongoDB",
			db: &domain.Database{
				Name:             "mymongo",
				Engine:           "mongodb",
				InternalHostname: "paas-db-mymongo",
			},
			expected: []string{"exec", "paas-db-mymongo", "mongosh", "--eval", "db.adminCommand('ping')"},
		},
		{
			name: "ClickHouse",
			db: &domain.Database{
				Name:             "myclick",
				Engine:           "clickhouse",
				InternalHostname: "paas-db-myclick",
			},
			expected: []string{"exec", "paas-db-myclick", "clickhouse-client", "--query", "SELECT 1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := buildReadinessProbeArgs(tt.db)
			if len(args) != len(tt.expected) {
				t.Fatalf("expected args %v, got %v", tt.expected, args)
			}
			for i := range args {
				if args[i] != tt.expected[i] {
					t.Errorf("arg[%d] = %s, want %s", i, args[i], tt.expected[i])
				}
			}
		})
	}
}
