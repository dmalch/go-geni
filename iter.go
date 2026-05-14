package geni

import (
	"context"
	"iter"
)

// paginate drives the page-by-page fetch loop shared by every Iter*
// method on Client. It calls fetchPage for successive 1-indexed pages,
// yields each result pointer to the caller's range loop, and stops
// when:
//
//   - fetchPage returns hasNext == false (end of data),
//   - fetchPage returns a non-nil error (yielded once, paired with a
//     nil item), or
//   - the caller breaks out of the range loop (yield returns false).
//
// The yielded value is a pointer into the page's Results slice — cheap
// to pass and lets callers read fields directly. Each page's slice is
// freshly allocated by the underlying Get* method, so the pointer is
// safe to hold across iterations.
func paginate[T any](
	ctx context.Context,
	fetchPage func(ctx context.Context, page int) (results []T, hasNext bool, err error),
) iter.Seq2[*T, error] {
	return func(yield func(*T, error) bool) {
		for page := 1; ; page++ {
			results, hasNext, err := fetchPage(ctx, page)
			if err != nil {
				yield(nil, err)
				return
			}
			for i := range results {
				if !yield(&results[i], nil) {
					return
				}
			}
			if !hasNext {
				return
			}
		}
	}
}
