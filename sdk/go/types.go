package sdk

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

// UserState mirrors the public JSON surface of core.UserState.
type UserState struct {
	UserID  string              `json:"user_id"`
	Points  map[string]int64    `json:"points"`
	Badges  map[string]struct{} `json:"badges"`
	Levels  map[string]int64    `json:"levels"`
	Updated time.Time           `json:"updated"`
}

// HealthStatus describes the /healthz response.
type HealthStatus struct {
	Status string                 `json:"status"`
	Checks map[string]interface{} `json:"checks"`
}

func decodeJSON(resp *http.Response, target any) error {
	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("request failed: status %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(target)
}

// ErrEmptyUserID is returned when user id is empty.
var ErrEmptyUserID = errors.New("user id is required")
