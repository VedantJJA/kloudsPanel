package http

import (
	"testing"

	"github.com/yourorg/klouds/api/internal/domain"
)

func TestResolveLinkValue(t *testing.T) {
	db := &domain.Database{
		ID:               "db-123",
		Name:             "main-postgres",
		Engine:           "postgres",
		InternalHostname: "paas-db-main-postgres",
		InternalPort:     5432,
		ResourceJSON: `{
			"internalConnectionUri": "postgresql://postgres:secret123@paas-db-main-postgres:5432/main_db?sslmode=disable",
			"externalConnectionUri": "postgresql://postgres:secret123@example.com:15432/main_db?sslmode=disable",
			"connectionUri": "postgresql://postgres:secret123@example.com:15432/main_db?sslmode=disable",
			"externalHost": "example.com",
			"externalPort": 15432,
			"username": "postgres",
			"password": "secret123",
			"databaseName": "main_db"
		}`,
	}

	tests := []struct {
		name     string
		link     *domain.ServiceDatabaseLink
		expected string
	}{
		{
			name: "Internal ConnectionString",
			link: &domain.ServiceDatabaseLink{
				ConnectionKind: domain.ConnectionInternal,
				Property:       "connectionString",
			},
			expected: "postgresql://postgres:secret123@paas-db-main-postgres:5432/main_db?sslmode=disable",
		},
		{
			name: "External ConnectionString",
			link: &domain.ServiceDatabaseLink{
				ConnectionKind: domain.ConnectionExternal,
				Property:       "connectionString",
			},
			expected: "postgresql://postgres:secret123@example.com:15432/main_db?sslmode=disable",
		},
		{
			name: "Internal Host",
			link: &domain.ServiceDatabaseLink{
				ConnectionKind: domain.ConnectionInternal,
				Property:       "host",
			},
			expected: "paas-db-main-postgres",
		},
		{
			name: "Internal Port",
			link: &domain.ServiceDatabaseLink{
				ConnectionKind: domain.ConnectionInternal,
				Property:       "port",
			},
			expected: "5432",
		},
		{
			name: "External Port",
			link: &domain.ServiceDatabaseLink{
				ConnectionKind: domain.ConnectionExternal,
				Property:       "port",
			},
			expected: "15432",
		},
		{
			name: "Username Property",
			link: &domain.ServiceDatabaseLink{
				ConnectionKind: domain.ConnectionInternal,
				Property:       "username",
			},
			expected: "postgres",
		},
		{
			name: "Password Property",
			link: &domain.ServiceDatabaseLink{
				ConnectionKind: domain.ConnectionInternal,
				Property:       "password",
			},
			expected: "secret123",
		},
		{
			name: "Database Name Property",
			link: &domain.ServiceDatabaseLink{
				ConnectionKind: domain.ConnectionInternal,
				Property:       "database",
			},
			expected: "main_db",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val := resolveLinkValue(db, tt.link)
			if val != tt.expected {
				t.Fatalf("resolveLinkValue() = %q, want %q", val, tt.expected)
			}
		})
	}
}

func TestExplicitLinksOverrideBlueprintAutoWiring(t *testing.T) {
	// 1. Setup pre-populated environment map as if from blueprint heuristic auto-wiring
	envMap := map[string]string{
		"PORT":         "8080",
		"NODE_ENV":     "production",
		"DATABASE_URL": "postgresql://old_user:old_pass@old_host:5432/old_db",
		"DB_HOST":      "old_heuristic_host",
	}

	// 2. Setup mock linked database and explicit links
	db := &domain.Database{
		ID:               "db-prod",
		Name:             "prod-db",
		Engine:           "postgres",
		InternalHostname: "paas-db-prod-db",
		InternalPort:     5432,
		ResourceJSON: `{
			"internalConnectionUri": "postgresql://realuser:realpass@paas-db-prod-db:5432/proddb?sslmode=disable",
			"username": "realuser",
			"password": "realpass",
			"databaseName": "proddb"
		}`,
	}

	links := []*domain.ServiceDatabaseLink{
		{
			ID:             "link-db-url",
			ServiceID:      "svc-app",
			DatabaseID:     db.ID,
			EnvVarName:     "DATABASE_URL",
			ConnectionKind: domain.ConnectionInternal,
			Property:       "connectionString",
		},
		{
			ID:             "link-db-host",
			ServiceID:      "svc-app",
			DatabaseID:     db.ID,
			EnvVarName:     "DB_HOST",
			ConnectionKind: domain.ConnectionInternal,
			Property:       "host",
		},
	}

	// 3. Run explicit link resolution logic (mirroring executeDeployment)
	for _, link := range links {
		resolvedVal := resolveLinkValue(db, link)
		if resolvedVal != "" {
			envMap[link.EnvVarName] = resolvedVal
		}
	}

	// 4. Assert explicit links won over old auto-wiring
	expectedURL := "postgresql://realuser:realpass@paas-db-prod-db:5432/proddb?sslmode=disable"
	if envMap["DATABASE_URL"] != expectedURL {
		t.Fatalf("DATABASE_URL = %q, want explicit link value %q", envMap["DATABASE_URL"], expectedURL)
	}

	expectedHost := "paas-db-prod-db"
	if envMap["DB_HOST"] != expectedHost {
		t.Fatalf("DB_HOST = %q, want explicit link value %q", envMap["DB_HOST"], expectedHost)
	}

	// Non-overridden variables remain intact
	if envMap["PORT"] != "8080" || envMap["NODE_ENV"] != "production" {
		t.Fatalf("unrelated env vars modified: %+v", envMap)
	}
}

func TestRedactSensitiveValue(t *testing.T) {
	rawURL := "postgresql://postgres:supersecretpassword@paas-db-test:5432/mydb?sslmode=disable"
	redactedURL := redactSensitiveValue(rawURL, "connectionString")
	expectedURL := "postgresql://postgres:********@paas-db-test:5432/mydb?sslmode=disable"
	if redactedURL != expectedURL {
		t.Fatalf("redactSensitiveValue(URL) = %q, want %q", redactedURL, expectedURL)
	}

	rawPass := "mySuperSecretPassword"
	redactedPass := redactSensitiveValue(rawPass, "password")
	if redactedPass != "********" {
		t.Fatalf("redactSensitiveValue(password) = %q, want ********", redactedPass)
	}

	host := "paas-db-test"
	notRedactedHost := redactSensitiveValue(host, "host")
	if notRedactedHost != host {
		t.Fatalf("redactSensitiveValue(host) = %q, want %q", notRedactedHost, host)
	}
}
