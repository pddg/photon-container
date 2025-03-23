package unarchiver

type UnarchiverOption func(*Unarchiver)

// WithUnarchiveLimitBytesPerSec sets the unarchive speed limit in bytes per second.
// The default is math.MaxFloat64.
func WithUnarchiveLimitBytesPerSec(limit float64) UnarchiverOption {
	return func(u *Unarchiver) {
		u.unarchiveLimitBytesPerSec = limit
	}
}
