package wt

import (
	"encoding/json"
	"testing"
)

func TestOptAbsentVsNull(t *testing.T) {
	// wt's documented semantics: absent = nothing to report, null =
	// requested but not determined. Opt must keep the two apart.
	type item struct {
		Integration Opt[struct {
			Reason string `json:"reason"`
		}] `json:"integration"`
	}
	tests := []struct {
		name        string
		data        string
		wantPresent bool
		wantNull    bool
		wantOK      bool
		wantReason  string
	}{
		{name: "absent", data: `{}`},
		{name: "null", data: `{"integration": null}`, wantPresent: true, wantNull: true},
		{name: "value", data: `{"integration": {"reason": "same_commit"}}`, wantPresent: true, wantOK: true, wantReason: "same_commit"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var it item
			if err := json.Unmarshal([]byte(tt.data), &it); err != nil {
				t.Fatal(err)
			}
			if it.Integration.Present != tt.wantPresent || it.Integration.Null != tt.wantNull {
				t.Errorf("Present=%v Null=%v, want Present=%v Null=%v",
					it.Integration.Present, it.Integration.Null, tt.wantPresent, tt.wantNull)
			}
			v, ok := it.Integration.Get()
			if ok != tt.wantOK || v.Reason != tt.wantReason {
				t.Errorf("Get() = (%+v, %v), want (reason=%q, %v)", v, ok, tt.wantReason, tt.wantOK)
			}
		})
	}
}
