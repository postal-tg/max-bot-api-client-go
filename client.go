package maxbot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

var (
	errLongPollTimeout = &TimeoutError{
		Op:     "long polling",
		Reason: "request timeout exceeded",
	}
)

type client struct {
	key        string
	version    string
	baseURL    *url.URL
	httpClient *http.Client
}

func newClient(key string, version string, baseURL *url.URL, httpClient *http.Client) *client {
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: defaultTimeout,
		}
	}

	return &client{
		key:        key,
		version:    version,
		baseURL:    baseURL,
		httpClient: httpClient,
	}
}

func (cl *client) createTimeoutError(op string, reason string) *TimeoutError {
	return &TimeoutError{
		Op:     op,
		Reason: reason,
	}
}

func (cl *client) request(ctx context.Context, method, path string, query url.Values, reset bool, body interface{}) (io.ReadCloser, error) {
	if body == nil {
		return cl.requestReader(ctx, method, path, query, reset, nil)
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, &SerializationError{
			Op:   "marshal",
			Type: "request body",
			Err:  err,
		}
	}

	return cl.requestReader(ctx, method, path, query, reset, bytes.NewReader(data))
}

func (cl *client) requestReader(ctx context.Context, method, path string, query url.Values, reset bool, body io.Reader) (io.ReadCloser, error) {
	if query == nil {
		query = url.Values{}
	}

	u := *cl.baseURL
	u.Path = path

	/* DEPRECATED
	if !reset {
		query.Set("access_token", cl.key)
	}
	*/

	query.Set("v", cl.version)
	u.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", fmt.Sprintf("max-bot-api-client-go/%s", cl.version))
	if !reset {
		req.Header.Set("Authorization", cl.key)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := cl.httpClient.Do(req)
	if err != nil {
		if urlErr, ok := err.(*url.Error); ok {
			if urlErr.Timeout() {
				return nil, cl.createTimeoutError(
					fmt.Sprintf("%s %s", method, path),
					fmt.Sprintf("request timeout exceeded (%v)", cl.httpClient.Timeout),
				)
			}
		}

		return nil, &NetworkError{
			Op:  fmt.Sprintf("%s %s", method, path),
			Err: err,
		}
	}

	if resp.StatusCode != http.StatusOK {
		// Read the whole response body so we can include it into the error.
		raw, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()

		msg, details := parseMAXErrorPayload(raw)
		if msg == "" {
			msg = http.StatusText(resp.StatusCode)
		}

		return nil, &APIError{
			Code:       resp.StatusCode,
			HTTPStatus: resp.Status,
			Method:     method,
			URL:        req.URL.String(),
			Message:    msg,
			Details:    details,
			RawBody:    string(raw),
		}
	}

	return resp.Body, nil
}

// Close closes the HTTP client
func (cl *client) Close() error {
	if transport, ok := cl.httpClient.Transport.(*http.Transport); ok {
		transport.CloseIdleConnections()
	}

	return nil
}


// parseMAXErrorPayload tries to extract a human-friendly message and extra details from MAX API error body.
// It is intentionally defensive: MAX may return different shapes depending on the endpoint.
func parseMAXErrorPayload(raw []byte) (message string, details string) {
	b := bytes.TrimSpace(raw)
	if len(b) == 0 {
		return "", ""
	}

	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		// not JSON — plain text
		return strings.TrimSpace(string(b)), ""
	}

	m, ok := v.(map[string]any)
	if !ok {
		// array or something else — return compact JSON as details
		j, _ := json.Marshal(v)
		return "", strings.TrimSpace(string(j))
	}

	// Common fields
	if s, ok := m["message"].(string); ok {
		message = strings.TrimSpace(s)
	}
	if message == "" {
		if s, ok := m["error"].(string); ok {
			message = strings.TrimSpace(s)
		}
	}
	if message == "" {
		if s, ok := m["code"].(string); ok {
			message = strings.TrimSpace(s)
		}
	}

	// Optional details / validation errors
	var parts []string
	if s, ok := m["details"].(string); ok && strings.TrimSpace(s) != "" {
		parts = append(parts, "details="+strings.TrimSpace(s))
	}
	if errs, ok := m["errors"]; ok && errs != nil {
		j, _ := json.Marshal(errs)
		s := strings.TrimSpace(string(j))
		if s != "" {
			parts = append(parts, "errors="+s)
		}
	}
	if len(parts) > 0 {
		details = strings.Join(parts, "; ")
	} else {
		// If no known keys — keep whole object, it helps debugging.
		j, _ := json.Marshal(m)
		details = strings.TrimSpace(string(j))
	}

	return message, details
}
