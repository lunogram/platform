package linkwrap

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// WrapPushLinks scans the push template data's "data" map for URL values
// and rewrites them with tracking URLs.
func WrapPushLinks(data json.RawMessage, key []byte, trackingURL string, projectID, campaignID, userID uuid.UUID) (json.RawMessage, error) {
	var push map[string]any
	if err := json.Unmarshal(data, &push); err != nil {
		return data, fmt.Errorf("parse push data: %w", err)
	}

	pushData, ok := push["data"].(map[string]any)
	if !ok {
		return data, nil
	}

	for k, v := range pushData {
		if s, ok := v.(string); ok && shouldWrapURL(s) {
			token, err := Encrypt(key, LinkPayload{
				ProjectID:  projectID,
				CampaignID: campaignID,
				UserID:     userID,
				URL:        s,
			})
			if err != nil {
				return data, fmt.Errorf("encrypt push link %q: %w", k, err)
			}
			pushData[k] = fmt.Sprintf("%s/c/%s", strings.TrimRight(trackingURL, "/"), token)
		}
	}

	push["data"] = pushData
	return json.Marshal(push)
}
