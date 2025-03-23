package photondata

import "strings"

type MigratorOption func(*Migrator)

func WithPhotonURL(url string) MigratorOption {
	if !strings.HasSuffix(url, "/") {
		url += "/"
	}
	return func(m *Migrator) {
		m.photonURL = url
	}
}
