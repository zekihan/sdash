package kubeinfo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// patchJSON sends a merge-patch to the API server.
func (client *Client) patchJSON(ctx context.Context, apiPath string, payload any) error {
	content, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("%w: marshal patch: %w", ErrBadResponse, err)
	}

	reqCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(
		reqCtx,
		http.MethodPatch,
		client.base+apiPath,
		bytes.NewReader(content),
	)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrUnavailable, err)
	}

	req.Header.Set("Content-Type", "application/merge-patch+json")
	req.Header.Set("Accept", "application/json")

	if client.token != "" {
		req.Header.Set("Authorization", "Bearer "+client.token)
	}

	resp, err := client.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	defer resp.Body.Close()

	_, copyErr := io.Copy(io.Discard, io.LimitReader(resp.Body, maxResponseBytes))
	_ = copyErr

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: PATCH %s returned %s", ErrUnavailable, apiPath, resp.Status)
	}

	return nil
}

// SyncArgoApplication triggers a sync the same way the Argo CD CLI does: by
// writing an operation onto the Application resource.
func (client *Client) SyncArgoApplication(ctx context.Context, name string) error {
	payload := map[string]any{
		"operation": map[string]any{
			"sync":        map[string]any{"prune": false},
			"initiatedBy": map[string]any{"username": "sdash", "automated": false},
			"info": []map[string]any{
				{"name": "Reason", "value": "Triggered from SDash dashboard"},
			},
		},
	}

	path := "/apis/argoproj.io/v1alpha1/namespaces/" + client.argocdNS +
		"/applications/" + url.PathEscape(name)

	return client.patchJSON(ctx, path, payload)
}

// RestartDeployment performs a kubectl-style rollout restart by stamping
// the pod template with the current time.
func (client *Client) RestartDeployment(ctx context.Context, namespace, name string) error {
	payload := map[string]any{
		"spec": map[string]any{
			"template": map[string]any{
				"metadata": map[string]any{
					"annotations": map[string]any{
						"kubectl.kubernetes.io/restartedAt": time.Now().Format(time.RFC3339),
						"sdash/restartedBy":                 "dashboard",
					},
				},
			},
		},
	}

	path := "/apis/apps/v1/namespaces/" + url.PathEscape(namespace) +
		"/deployments/" + url.PathEscape(name)

	return client.patchJSON(ctx, path, payload)
}
