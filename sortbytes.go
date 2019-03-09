package art

// SortBytes implements sorting a []byte. It sorts in place and uses a very
// basic algorithm under the assumption that we only need this for small byte
// arrays (4 or 16 bytes max) which fit in a cache line and so simple sorts with
// bad big-O are quicker due to having fewer branches etc. This is based on the
// insertion sort implementation from Go's `sort` package but without the
// interface indirection cost.
func SortBytes(b []byte) {
	for i := 1; i < len(b); i++ {
		for j := i; j > 0 && b[j] < b[j-1]; j-- {
			b[j], b[j-1] = b[j-1], b[j]
		}
	}
}
