package sdk

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"gamifykit/core"
)

// Option configures the Client.
type Option func(*Client)

// Client provides typed access to the GamifyKit HTTP + WebSocket API.
type Client struct {
	baseURL    string
	wsURL      string
	httpClient *http.Client
	headers    http.Header
}

// NewClient constructs a new SDK client targeting the given baseURL (e.g., http://localhost:8080/api).
func NewClient(baseURL string, opts ...Option) (*Client, error) {
	if strings.TrimSpace(baseURL) == "" {
		return nil, errors.New("baseURL is required")
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	c := &Client{
		baseURL:    baseURL,
		wsURL:      deriveWSURL(baseURL),
		httpClient: http.DefaultClient,
		headers:    make(http.Header),
	}
	for _, opt := range opts {
		opt(c)
	}
	return c, nil
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(h *http.Client) Option {
	return func(c *Client) {
		if h != nil {
			c.httpClient = h
		}
	}
}

// WithAuthToken adds an Authorization: Bearer token header to all requests (HTTP + WS).
func WithAuthToken(token string) Option {
	return func(c *Client) {
		if strings.TrimSpace(token) != "" {
			c.headers.Set("Authorization", "Bearer "+token)
		}
	}
}

// WithAPIKey adds an X-API-Key header.
func WithAPIKey(key string) Option {
	return func(c *Client) {
		if strings.TrimSpace(key) != "" {
			c.headers.Set("X-API-Key", key)
		}
	}
}

// WithHeader sets an arbitrary header applied to HTTP and WS calls.
func WithHeader(k, v string) Option {
	return func(c *Client) {
		if k != "" {
			c.headers.Set(k, v)
		}
	}
}

// AddPoints increments the given metric (default xp) for a user and returns the new total.
func (c *Client) AddPoints(ctx context.Context, userID string, delta int64, metric string) (int64, error) {
	if strings.TrimSpace(userID) == "" {
		return 0, ErrEmptyUserID
	}
	if metric == "" {
		metric = string(core.MetricXP)
	}

	u, err := url.Parse(fmt.Sprintf("%s/users/%s/points", c.baseURL, url.PathEscape(userID)))
	if err != nil {
		return 0, err
	}
	q := u.Query()
	q.Set("metric", metric)
	q.Set("delta", fmt.Sprintf("%d", delta))
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), nil)
	if err != nil {
		return 0, err
	}
	c.applyHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var body struct {
		Total int64   `json:"total"`
		Err   *string `json:"err"`
	}
	if err := decodeJSON(resp, &body); err != nil {
		return 0, err
	}
	if body.Err != nil && *body.Err != "" {
		return 0, errors.New(*body.Err)
	}
	return body.Total, nil
}

// AwardBadge assigns a badge to a user.
func (c *Client) AwardBadge(ctx context.Context, userID string, badge string) error {
	if strings.TrimSpace(userID) == "" {
		return ErrEmptyUserID
	}
	u := fmt.Sprintf("%s/users/%s/badges/%s", c.baseURL, url.PathEscape(userID), url.PathEscape(badge))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, nil)
	if err != nil {
		return err
	}
	c.applyHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var body struct {
		OK  bool    `json:"ok"`
		Err *string `json:"err"`
	}
	if err := decodeJSON(resp, &body); err != nil {
		return err
	}
	if body.Err != nil && *body.Err != "" {
		return errors.New(*body.Err)
	}
	if !body.OK {
		return errors.New("badge not awarded")
	}
	return nil
}

// GetUser fetches the current gamification state for a user.
func (c *Client) GetUser(ctx context.Context, userID string) (UserState, error) {
	if strings.TrimSpace(userID) == "" {
		return UserState{}, ErrEmptyUserID
	}
	u := fmt.Sprintf("%s/users/%s", c.baseURL, url.PathEscape(userID))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return UserState{}, err
	}
	c.applyHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return UserState{}, err
	}
	defer resp.Body.Close()

	var st UserState
	if err := decodeJSON(resp, &st); err != nil {
		return UserState{}, err
	}
	return st, nil
}

// Health probes /healthz and returns status + storage check.
func (c *Client) Health(ctx context.Context) (HealthStatus, error) {
	u := c.baseURL + "/healthz"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return HealthStatus{}, err
	}
	c.applyHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return HealthStatus{}, err
	}
	defer resp.Body.Close()

	var hs HealthStatus
	if err := decodeJSON(resp, &hs); err != nil {
		return HealthStatus{}, err
	}
	return hs, nil
}

// SubscribeEvents connects to the WebSocket stream and emits core.Event values.
// The returned channel closes when ctx is done or the connection drops.
func (c *Client) SubscribeEvents(ctx context.Context) (<-chan core.Event, error) {
	if c.wsURL == "" {
		return nil, errors.New("wsURL is not set; ensure baseURL is http/https")
	}
	dialer := websocket.Dialer{
		HandshakeTimeout: 5 * time.Second,
	}
	conn, _, err := dialer.DialContext(ctx, c.wsURL, c.headers)
	if err != nil {
		return nil, err
	}

	out := make(chan core.Event, 32)
	go func() {
		defer close(out)
		defer conn.Close()
		for {
			select {
			case <-ctx.Done():
				return
			default:
				var evt core.Event
				if err := conn.ReadJSON(&evt); err != nil {
					return
				}
				select {
				case out <- evt:
				default:
					// drop if consumer is slow
				}
			}
		}
	}()
	return out, nil
}

func (c *Client) applyHeaders(r *http.Request) {
	for k, vals := range c.headers {
		for _, v := range vals {
			r.Header.Add(k, v)
		}
	}
}

func deriveWSURL(httpBase string) string {
	u, err := url.Parse(httpBase)
	if err != nil {
		return ""
	}
	switch u.Scheme {
	case "https":
		u.Scheme = "wss"
	case "http":
		u.Scheme = "ws"
	default:
		// leave as-is for custom schemes
	}
	u.Path = strings.TrimSuffix(u.Path, "/") + "/ws"
	return u.String()
}
