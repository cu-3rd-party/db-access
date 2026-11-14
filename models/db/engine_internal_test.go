package db

import "testing"

func TestSingleRoundTripDSN(t *testing.T) {
	tests := []struct {
		name, dsn, want string
	}{
		{"url без параметров", "postgres://u:p@h:5432/db", "postgres://u:p@h:5432/db?binary_parameters=yes"},
		{"url с параметрами", "postgres://u:p@h/db?sslmode=disable", "postgres://u:p@h/db?binary_parameters=yes&sslmode=disable"},
		{"уже включено", "postgres://h/db?binary_parameters=yes", "postgres://h/db?binary_parameters=yes"},
		{"key=value", "host=h dbname=db", "host=h dbname=db binary_parameters=yes"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := singleRoundTripDSN(tc.dsn)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}
