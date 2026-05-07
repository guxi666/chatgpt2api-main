package version

import "testing"

func TestGet(t *testing.T) {
	original := Version
	t.Cleanup(func() { Version = original })

	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "normalizes whitespace", raw: " 1.2.3 \n", want: "1.2.3"},
		{name: "falls back when empty", raw: "", want: "0.0.0-dev"},
		{name: "falls back when whitespace", raw: " \t", want: "0.0.0-dev"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Version = tt.raw
			if got := Get(); got != tt.want {
				t.Fatalf("Get() = %q, want %q", got, tt.want)
			}
		})
	}
}
