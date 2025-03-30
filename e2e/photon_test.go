package e2e_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/pddg/photon-container/internal/client/photonwrapper"
	"github.com/pddg/photon-container/internal/photondata"
)

type Geometry struct {
	Coordinates []float64 `json:"coordinates"`
	Type        string    `json:"type"`
}

type Feature struct {
	Geometry   Geometry       `json:"geometry"`
	Type       string         `json:"type"`
	Properties map[string]any `json:"properties"`
}

type ReverseGeocodeResponse struct {
	Type     string    `json:"type"`
	Features []Feature `json:"features"`
}

func waitUntilPhotonReady(t *testing.T, photonUrl, photonAgentUrl string) {
	t.Helper()
	require.Eventually(t, func() bool {
		// Wait until /migrate/status returns migrated
		out, err := unsafeBastionExec(t, "curl", "-sS", "-X", "GET", photonAgentUrl+"migrate/status")
		if err != nil {
			t.Logf("failed to execute command: %v", err)
			return false
		}
		var resp photonwrapper.MigrateStatusResponse
		if err := json.Unmarshal(out, &resp); err != nil {
			t.Logf("failed to unmarshal response: %v", err)
			return false
		}
		if resp.State != photondata.MigrationStateMigrated {
			t.Logf("Photon not ready yet, got state %s", resp.State)
			return false
		}
		t.Logf("Migration completed successfully")
		return true
	}, 1*time.Minute, 2*time.Second)

	require.Eventually(t, func() bool {
		// Wait until /status returns 200
		out, err := unsafeBastionExec(t, "curl", "-sS", "-X", "GET", photonUrl+"status")
		if err != nil {
			t.Logf("failed to execute command: %v", err)
			return false
		}
		resp := struct {
			Status string `json:"status"`
		}{}
		if err := json.Unmarshal(out, &resp); err != nil {
			t.Logf("failed to unmarshal response: %v", err)
			return false
		}
		if resp.Status != "Ok" {
			t.Logf("Photon not ready yet, got status %s", resp.Status)
			return false
		}
		t.Logf("Photon is ready")
		return true
	}, 1*time.Minute, 2*time.Second)
}

func reverseGeocode(t *testing.T, photonUrl string) ReverseGeocodeResponse {
	t.Helper()
	out := bastionExec(t, "curl", "-sS", "-X", "GET", photonUrl+"reverse?lat=42.508004&lon=1.529161")
	var resp ReverseGeocodeResponse
	if err := json.Unmarshal(out, &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	return resp
}

func assertReverseGeocodeResponse(t *testing.T, resp ReverseGeocodeResponse) {
	t.Helper()
	if len(resp.Features) == 0 {
		t.Fatalf("expected at least one feature, got %d", len(resp.Features))
	}
	if resp.Features[0].Type != "Feature" {
		t.Fatalf("expected feature type to be 'Feature', got '%s'", resp.Features[0].Type)
	}
	if resp.Features[0].Geometry.Type != "Point" {
		t.Fatalf("expected geometry type to be 'Point', got '%s'", resp.Features[0].Geometry.Type)
	}
}
