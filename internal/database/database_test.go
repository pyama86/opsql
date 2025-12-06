package database

import (
	"os"
	"testing"
)

func TestGetQueryTimeout(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		want     int
	}{
		{
			name:     "デフォルト値（環境変数未設定）",
			envValue: "",
			want:     30,
		},
		{
			name:     "環境変数で10秒を設定",
			envValue: "10",
			want:     10,
		},
		{
			name:     "環境変数で60秒を設定",
			envValue: "60",
			want:     60,
		},
		{
			name:     "不正な値（文字列）",
			envValue: "invalid",
			want:     30,
		},
		{
			name:     "不正な値（負の数）",
			envValue: "-5",
			want:     30,
		},
		{
			name:     "不正な値（ゼロ）",
			envValue: "0",
			want:     30,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue == "" {
				_ = os.Unsetenv("OPSQL_QUERY_TIMEOUT")
			} else {
				t.Setenv("OPSQL_QUERY_TIMEOUT", tt.envValue)
			}

			got := getQueryTimeout()
			if got != tt.want {
				t.Errorf("getQueryTimeout() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDetectDriver(t *testing.T) {
	tests := []struct {
		name    string
		dsn     string
		want    string
		wantErr bool
	}{
		{
			name:    "MySQL DSN with mysql:// prefix",
			dsn:     "mysql://user:pass@localhost:3306/db",
			want:    "mysql",
			wantErr: false,
		},
		{
			name:    "MySQL DSN with tcp format",
			dsn:     "user:pass@tcp(localhost:3306)/db",
			want:    "mysql",
			wantErr: false,
		},
		{
			name:    "PostgreSQL DSN with postgres:// prefix",
			dsn:     "postgres://user:pass@localhost:5432/db",
			want:    "postgres",
			wantErr: false,
		},
		{
			name:    "PostgreSQL DSN with postgresql:// prefix",
			dsn:     "postgresql://user:pass@localhost:5432/db",
			want:    "postgres",
			wantErr: false,
		},
		{
			name:    "Unsupported driver",
			dsn:     "sqlite://test.db",
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := detectDriver(tt.dsn)
			if (err != nil) != tt.wantErr {
				t.Errorf("detectDriver() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("detectDriver() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConvertDSN(t *testing.T) {
	tests := []struct {
		name   string
		dsn    string
		driver string
		want   string
	}{
		{
			name:   "MySQL with mysql:// prefix",
			dsn:    "mysql://user:pass@localhost:3306/db",
			driver: "mysql",
			want:   "user:pass@localhost:3306/db",
		},
		{
			name:   "MySQL without prefix",
			dsn:    "user:pass@tcp(localhost:3306)/db",
			driver: "mysql",
			want:   "user:pass@tcp(localhost:3306)/db",
		},
		{
			name:   "PostgreSQL DSN",
			dsn:    "postgres://user:pass@localhost:5432/db",
			driver: "postgres",
			want:   "postgres://user:pass@localhost:5432/db",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := convertDSN(tt.dsn, tt.driver)
			if err != nil {
				t.Errorf("convertDSN() error = %v", err)
				return
			}
			if got != tt.want {
				t.Errorf("convertDSN() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMaskSecret(t *testing.T) {
	tests := []struct {
		name string
		dsn  string
		want string
	}{
		{
			name: "MySQL DSN",
			dsn:  "mysql://user:password@localhost:3306/db",
			want: "mysql://user:***@localhost:3306/db",
		},
		{
			name: "PostgreSQL DSN",
			dsn:  "postgres://admin:secret123@localhost:5432/mydb",
			want: "postgres://admin:***@localhost:5432/mydb",
		},
		{
			name: "DSN without password",
			dsn:  "postgres://localhost:5432/db",
			want: "postgres://localhost:5432/db",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MaskSecret(tt.dsn)
			if got != tt.want {
				t.Errorf("MaskSecret() = %v, want %v", got, tt.want)
			}
		})
	}
}
