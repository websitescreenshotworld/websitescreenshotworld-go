package wsw

// String returns a pointer to v, for optional string fields.
func String(v string) *string { return &v }

// Bool returns a pointer to v, for optional boolean fields.
func Bool(v bool) *bool { return &v }

// Int returns a pointer to v, for optional integer fields.
func Int(v int) *int { return &v }

// Float64 returns a pointer to v, for optional number fields.
func Float64(v float64) *float64 { return &v }
