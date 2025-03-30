package photonagent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/pddg/photon-container/internal/logging"
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

func (c *Client) MigrateStart(ctx context.Context, archivePath string, options ...UploadOption) error {
	opts := initUploadOptions(options...)
	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("photonagent.Client.MigrateStart: failed to open %q: %w", archivePath, err)
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil {
		return fmt.Errorf("photonagent.Client.MigrateStart: failed to get file size: %w", err)
	}
	p := NewProgress(ctx, f, stat.Size(), opts.progressInterval, logging.FromContext(ctx))
	defer p.Stop()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"migrate/upload", p)
	if err != nil {
		return err
	}
	req.URL.RawQuery = opts.toQuery().Encode()
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	bodyByte, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("photonagent.Client.MigrateStart: failed to read response body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("photonagent.Client.MigrateStart: unexpected status code: %d %s", resp.StatusCode, string(bodyByte))
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
		return nil, fmt.Errorf("photonagent.Client.WaitForMigrate: failed to read response body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("photonagent.Client.WaitForMigrate: unexpected status code: %d %s", resp.StatusCode, string(bodyByte))
	}

	var res *MigrateStatusResponse
	if err := json.Unmarshal(bodyByte, &res); err != nil {
		return nil, fmt.Errorf("photonagent.Client.WaitForMigrate: failed to unmarshal response body: %w", err)
	}
	return res, nil
}

func (c *Client) ResetStatus(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.baseURL+"migrate/status", nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("photonagent.Client.ResetStatus: failed to send request: %w", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("photonagent.Client.ResetStatus: unexpected status code: %d", resp.StatusCode)
	}
	return nil
}
