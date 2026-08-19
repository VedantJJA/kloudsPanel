package system

import (
	"strings"
	"testing"
)

func TestBuildDatabaseRunArgs(t *testing.T) {
	tests := []struct {
		name         string
		engine       string
		defaultUser  string
		password     string
		dbName       string
		expectedImg  string
		expectedEnv  string
		expectedVol  string
	}{
		{
			name:        "Postgres with standard case",
			engine:      "postgres",
			defaultUser: "postgres",
			password:    "secret123",
			dbName:      "myapp_db",
			expectedImg: "postgres:16-alpine",
			expectedEnv: "POSTGRES_PASSWORD=secret123",
			expectedVol: "paas-db-data-test-db:/var/lib/postgresql/data",
		},
		{
			name:        "PostgreSQL mixed case synonym",
			engine:      "POSTGRESQL",
			defaultUser: "postgres",
			password:    "pgpass",
			dbName:      "testdb",
			expectedImg: "postgres:16-alpine",
			expectedEnv: "POSTGRES_PASSWORD=pgpass",
			expectedVol: "paas-db-data-test-db:/var/lib/postgresql/data",
		},
		{
			name:        "Postgres empty engine defaults to postgres with credentials and volume",
			engine:      "",
			defaultUser: "postgres",
			password:    "fallbackpass",
			dbName:      "fallbackdb",
			expectedImg: "postgres:16-alpine",
			expectedEnv: "POSTGRES_PASSWORD=fallbackpass",
			expectedVol: "paas-db-data-test-db:/var/lib/postgresql/data",
		},
		{
			name:        "Mongo synonym to mongodb",
			engine:      "Mongo",
			defaultUser: "admin",
			password:    "mongosecret",
			dbName:      "appdb",
			expectedImg: "mongo:7.0",
			expectedEnv: "MONGO_INITDB_ROOT_PASSWORD=mongosecret",
			expectedVol: "paas-db-data-test-db:/data/db",
		},
		{
			name:        "MySQL uppercase",
			engine:      "MYSQL",
			defaultUser: "root",
			password:    "mysqlroot",
			dbName:      "mydb",
			expectedImg: "mysql:8.0",
			expectedEnv: "MYSQL_ROOT_PASSWORD=mysqlroot",
			expectedVol: "paas-db-data-test-db:/var/lib/mysql",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := BuildDatabaseRunArgs(tt.engine, "paas-db-test-db", "test-db", tt.defaultUser, tt.password, tt.dbName, 15432, 5432)
			joined := strings.Join(args, " ")

			if !strings.Contains(joined, tt.expectedImg) {
				t.Errorf("expected image %q in args, got: %s", tt.expectedImg, joined)
			}
			if !strings.Contains(joined, tt.expectedEnv) {
				t.Errorf("expected env %q in args, got: %s", tt.expectedEnv, joined)
			}
			if !strings.Contains(joined, tt.expectedVol) {
				t.Errorf("expected volume %q in args, got: %s", tt.expectedVol, joined)
			}
		})
	}
}
