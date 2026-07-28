package wt

import "testing"

func TestPRRef(t *testing.T) {
	tests := []struct {
		input   string
		want    string
		wantErr bool
	}{
		{input: "123", want: "pr:123"},
		{input: " 123 ", want: "pr:123"},
		{input: "#123", want: "pr:123"},
		{input: "pr:123", want: "pr:123"},
		{input: "PR:123", want: "pr:123"},
		{input: "mr:45", want: "mr:45"},
		{input: "https://github.com/o/r/pull/123", want: "https://github.com/o/r/pull/123"},
		{input: "http://gitlab.example/o/r/-/merge_requests/9", want: "http://gitlab.example/o/r/-/merge_requests/9"},
		{input: "", wantErr: true},
		{input: "#", wantErr: true},
		{input: "feat-a", wantErr: true},
		{input: "pr:abc", wantErr: true},
		{input: "pr:", wantErr: true},
		{input: "12a", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := PRRef(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("PRRef(%q) should fail, got %q", tt.input, got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("PRRef(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
