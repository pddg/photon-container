package e2e_test

import (
	"testing"
)

func Test_ServerSideDownload(t *testing.T) {
	t.Parallel()
	ns := "server-side-dl"
	// Setup
	photonUrl, photonAgentUrl := setup(t, ns)

	// Exercise
	bastionExec(t, "curl", "-sS", "-X", "POST", photonAgentUrl+"migrate/download")

	// Verify
	waitUntilPhotonReady(t, photonUrl, photonAgentUrl)
	resp := reverseGeocode(t, photonUrl)
	assertReverseGeocodeResponse(t, resp)
}

func Test_UploadFromClientSide(t *testing.T) {
	t.Parallel()
	ns := "client-side-dl"
	// Setup
	photonUrl, photonAgentUrl := setup(t, ns)

	// Exercise
	bastionExec(t,
		"/bin/photon-db-uploader",
		"-download-to", "/tmp/photon-db.tar.bz2",
		"-photon-wrapper-url", photonAgentUrl,
		"-database-country-code", "ad",
	)

	// Verify
	waitUntilPhotonReady(t, photonUrl, photonAgentUrl)
	resp := reverseGeocode(t, photonUrl)
	assertReverseGeocodeResponse(t, resp)
}

func Test_UploadUncompressedFromClientSide(t *testing.T) {
	t.Parallel()
	ns := "upload-uncompressed"
	// Setup
	photonUrl, photonAgentUrl := setup(t, ns)

	// Exercise
	bastionExec(t,
		"/bin/photon-db-uploader",
		"-download-to", "/tmp/client-uncompressed.tar.bz2",
		"-download-only",
		"-database-country-code", "ad",
	)
	bastionExec(t,
		"bzip2", "-d", "/tmp/client-uncompressed.tar.bz2",
	)
	bastionExec(t,
		"/bin/photon-db-uploader",
		"-photon-wrapper-url", photonAgentUrl,
		"-archive", "/tmp/client-uncompressed.tar",
		"-no-compressed",
		"-wait",
	)

	// Verify
	waitUntilPhotonReady(t, photonUrl, photonAgentUrl)
	resp := reverseGeocode(t, photonUrl)
	assertReverseGeocodeResponse(t, resp)
}
