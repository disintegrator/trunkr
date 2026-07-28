package herdr

import "testing"

func TestParseContext(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    InvocationContext
		wantDir string
		wantErr bool
	}{
		{
			name:    "empty payload is a valid empty context",
			raw:     "",
			wantDir: "",
		},
		{
			name: "focused pane cwd wins",
			raw:  `{"workspace_id":"w1","workspace_cwd":"/repo","focused_pane_id":"w1:p1","focused_pane_cwd":"/repo/sub","tab_id":"w1:t1"}`,
			want: InvocationContext{WorkspaceID: "w1", WorkspaceCwd: "/repo",
				FocusedPaneID: "w1:p1", FocusedPaneCwd: "/repo/sub", TabID: "w1:t1"},
			wantDir: "/repo/sub",
		},
		{
			name:    "falls back to workspace cwd",
			raw:     `{"workspace_id":"w1","workspace_cwd":"/repo","focused_pane_cwd":null}`,
			want:    InvocationContext{WorkspaceID: "w1", WorkspaceCwd: "/repo"},
			wantDir: "/repo",
		},
		{
			name:    "all nulls decode to empty",
			raw:     `{"workspace_id":null,"workspace_cwd":null,"tab_id":null,"focused_pane_id":null,"focused_pane_cwd":null}`,
			wantDir: "",
		},
		{
			name:    "garbage is an error",
			raw:     "not json",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseContext(tt.raw)
			if tt.wantErr {
				if err == nil {
					t.Fatal("want error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("context = %+v, want %+v", got, tt.want)
			}
			if dir := got.TargetDir(); dir != tt.wantDir {
				t.Errorf("TargetDir() = %q, want %q", dir, tt.wantDir)
			}
		})
	}
}
