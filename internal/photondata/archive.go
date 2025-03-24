package photondata

import (
	"fmt"
	"strings"
)

type Archive struct {
	countryCode string
	baseURL     string
	archiveName string
}

func NewArchive(baseURL string, options ...ArchiveOption) Archive {
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}
	a := Archive{
		baseURL: baseURL,
	}
	for _, option := range options {
		option(&a)
	}
	return a
}

func (a *Archive) String() string {
	return a.URL()
}

func (a Archive) Name() string {
	if a.archiveName != "" {
		return a.archiveName
	}
	if a.countryCode == "" {
		return "photon-db-latest.tar.bz2"
	}
	return fmt.Sprintf("photon-db-%s-latest.tar.bz2", a.countryCode)
}

func (a Archive) BaseURL() string {
	if a.countryCode == "" {
		return a.baseURL
	}
	return a.baseURL + fmt.Sprintf("extracts/by-country-code/%s/", a.countryCode)
}

func (a Archive) URL() string {
	return a.BaseURL() + a.Name()
}

func (a Archive) FromArchiveName(name string) Archive {
	a.archiveName = name
	return a
}

type ArchiveOption func(*Archive)

func WithCountryCode(countryCode string) ArchiveOption {
	return func(a *Archive) {
		a.countryCode = countryCode
	}
}

func WithArchiveName(name string) ArchiveOption {
	return func(a *Archive) {
		a.archiveName = name
	}
}
