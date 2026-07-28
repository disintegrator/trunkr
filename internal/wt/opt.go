package wt

import "encoding/json"

// Opt distinguishes wt's documented "no value" semantics: absent means
// nothing to report (not applicable, not requested, determined-empty), while
// null means requested but not determined (timeout, forge failure) — the
// JSON form of the table's · placeholder.
type Opt[T any] struct {
	// Present is true when the field appeared in the JSON at all.
	Present bool
	// Null is true when the field appeared as JSON null.
	Null  bool
	Value T
}

func (o *Opt[T]) UnmarshalJSON(data []byte) error {
	o.Present = true
	if string(data) == "null" {
		o.Null = true
		return nil
	}
	return json.Unmarshal(data, &o.Value)
}

// Get returns the value and whether it was actually determined (present and
// not null).
func (o Opt[T]) Get() (T, bool) {
	return o.Value, o.Present && !o.Null
}
