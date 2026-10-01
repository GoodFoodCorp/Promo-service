package postgres

import "testing"

func TestMigrationURLSupportsPostgresSchemes(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{
			name: "postgres",
			url:  "postgres://promo:secret@promo-db:5432/promo_db",
			want: "pgx5://promo:secret@promo-db:5432/promo_db",
		},
		{
			name: "postgresql",
			url:  "postgresql://promo:secret@promo-db:5432/promo_db",
			want: "pgx5://promo:secret@promo-db:5432/promo_db",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := migrationURL(tt.url); got != tt.want {
				t.Fatalf("migrationURL() = %q, want %q", got, tt.want)
			}
		})
	}
}