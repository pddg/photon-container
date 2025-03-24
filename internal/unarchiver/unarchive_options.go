package unarchiver

type UnarchiveOption func(*runtimeOption)

// NoCompression represents an option that the archive is not compressed.
func NoCompression() UnarchiveOption {
	return func(a *runtimeOption) {
		a.noCompression = true
	}
}
