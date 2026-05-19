package geni

// errInvalidArg is a thin wrapper so client-side argument errors
// produce a consistent message prefix without leaking the helper
// type to callers. Kept in root only while the video resource still
// lives here — moves into video/ alongside the methods later.
type errInvalidArg string

func (e errInvalidArg) Error() string { return string(e) }
