package oapi

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/ptr"
	"github.com/lunogram/platform/internal/pubsub/schemas"
)

// dataToRawMessage marshals an optional map into json.RawMessage. Returns nil
// for a nil pointer.
func dataToRawMessage(data *map[string]any) (json.RawMessage, error) {
	if data == nil {
		return nil, nil
	}
	raw, err := json.Marshal(*data)
	if err != nil {
		return nil, fmt.Errorf("marshal data: %w", err)
	}
	return raw, nil
}

// ToInboxMessage converts an InboxMessageCreate request into a pubsub InboxMessage schema.
func (item InboxMessageCreate) ToInboxMessage(projectID uuid.UUID) (schemas.InboxMessage, error) {
	var content json.RawMessage
	if item.Content != nil {
		content = *item.Content
	}
	data, err := dataToRawMessage(item.Data)
	if err != nil {
		return schemas.InboxMessage{}, err
	}
	return schemas.InboxMessage{
		ProjectID:        projectID,
		ExternalID:       ExternalIDString(item.Identifier),
		Channel:          string(item.Channel),
		Identifiers:      ToParams(item.Target),
		SenderIdentityID: item.SenderIdentityId,
		CampaignID:       item.CampaignId,
		BroadcastID:      item.BroadcastId,
		Content:          content,
		Data:             data,
		Tags:             ptr.FromOr(item.Tags, []string{}),
		Priority:         item.Priority,
		Source:           item.Source,
		ScheduledAt:      item.ScheduledAt,
		ExpiresAt:        item.ExpiresAt,
	}, nil
}

// ToInboxMessage converts an OrganizationInboxMessageCreate request into a pubsub InboxMessage schema.
func (item OrganizationInboxMessageCreate) ToInboxMessage(projectID uuid.UUID) (schemas.InboxMessage, error) {
	var content json.RawMessage
	if item.Content != nil {
		content = *item.Content
	}
	data, err := dataToRawMessage(item.Data)
	if err != nil {
		return schemas.InboxMessage{}, err
	}
	return schemas.InboxMessage{
		ProjectID:        projectID,
		ExternalID:       ExternalIDString(item.Identifier),
		Channel:          string(item.Channel),
		Identifiers:      ToParams(item.Target),
		SenderIdentityID: item.SenderIdentityId,
		CampaignID:       item.CampaignId,
		BroadcastID:      item.BroadcastId,
		Content:          content,
		Data:             data,
		Tags:             ptr.FromOr(item.Tags, []string{}),
		Priority:         item.Priority,
		Source:           item.Source,
		ScheduledAt:      item.ScheduledAt,
		ExpiresAt:        item.ExpiresAt,
	}, nil
}
