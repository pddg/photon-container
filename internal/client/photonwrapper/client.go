package photonwrapper

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/pddg/photon-container/internal/photondata"
)

type Client struct {
	httpClient *http.Client
	baseURL    string
}

func NewClient(
	httpClient *http.Client,
	baseURL string,
) *Client {
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}
	return &Client{
		httpClient: httpClient,
		baseURL:    baseURL,
	}
}

func (c *Client) MigrateStart(ctx context.Context, archivePath string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("photonwrapper.Client.MigrateStart: failed to open %q: %w", archivePath, err)
	}
	defer f.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"migrate", f)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	bodyByte, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("photonwrapper.Client.MigrateStart: failed to read response body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("photonwrapper.Client.MigrateStart: unexpected status code: %d %s", resp.StatusCode, string(bodyByte))
	}
	return nil
}

type MigrateStatusResponse struct {
	State   photondata.MigrationState `json:"state"`
	Version string                    `json:"version"`
}

func (c *Client) MigrateStatus(ctx context.Context) (*MigrateStatusResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"migrate/status", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	bodyByte, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("photonwrapper.Client.WaitForMigrate: failed to read response body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("photonwrapper.Client.WaitForMigrate: unexpected status code: %d %s", resp.StatusCode, string(bodyByte))
	}

	var res *MigrateStatusResponse
	if err := json.Unmarshal(bodyByte, &res); err != nil {
		return nil, fmt.Errorf("photonwrapper.Client.WaitForMigrate: failed to unmarshal response body: %w", err)
	}
	return res, nil
}
