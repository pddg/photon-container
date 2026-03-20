package photondata

import (
	"fmt"
	"net/url"
	"path"
)

const (
	DefaultDatabaseURL = "https://download1.graphhopper.com/public/photon-db-planet-1.0-latest.tar.bz2"
)

type Archive struct {
	baseURL     string
	archiveName string
}

func NewArchive(archiveURL string, options ...ArchiveOption) (Archive, error) {
	u, err := url.Parse(archiveURL)
	if err != nil {
		return Archive{}, fmt.Errorf("photondata.NewArchive: %w", err)
	}
	file := path.Base(u.Path)
	dir := path.Dir(u.Path)
	if dir == "." {
		dir = ""
	}
	a := Archive{
		baseURL:     u.Scheme + "://" + u.Host + dir,
		archiveName: file,
	}
	for _, option := range options {
		option(&a)
	}
	return a, nil
}

func (a *Archive) String() string {
	return a.URL()
}

func (a Archive) Name() string {
	return a.archiveName
}

func (a Archive) BaseURL() string {
	return a.baseURL
}

func (a Archive) URL() string {
	return a.BaseURL() + "/" + a.Name()
}

func (a Archive) FromArchiveName(name string) Archive {
	a.archiveName = name
	return a
}

type ArchiveOption func(*Archive)

func WithArchiveName(name string) ArchiveOption {
	return func(a *Archive) {
		a.archiveName = name
	}
}
