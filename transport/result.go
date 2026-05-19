package transport

// Result is the small JSON envelope Geni returns from no-content
// mutation endpoints (Delete*, follow / unfollow, tag, untag, …) —
// usually `{"result":"OK"}`. Shared across resources so each delete
// path doesn't redeclare an identical type.
type Result struct {
	Result string `json:"result,omitempty"`
}
