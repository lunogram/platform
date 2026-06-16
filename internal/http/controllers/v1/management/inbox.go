package v1

import (
	"encoding/json"
	"fmt"

	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/ptr"
	"github.com/lunogram/platform/internal/store/subjects"
	"github.com/lunogram/platform/pkg/modules"
)

func inboxMessageParams(body oapi.CreateInboxMessageRequest) (subjects.InboxMessageParams, error) {
	data, err := dataToRawMessage(body.Data)
	if err != nil {
		return subjects.InboxMessageParams{}, err
	}
	return subjects.InboxMessageParams{
		ExternalID:       inboxExternalID(body.Identifier),
		Channel:          modules.Channel(body.Channel),
		SenderIdentityID: body.SenderIdentityId,
		CampaignID:       body.CampaignId,
		BroadcastID:      body.BroadcastId,
		Content:          ptr.From(body.Content),
		Data:             data,
		Tags:             ptr.FromOr(body.Tags, []string{}),
		Priority:         body.Priority,
		Source:           body.Source,
		ScheduledAt:      body.ScheduledAt,
		ExpiresAt:        body.ExpiresAt,
	}, nil
}

func inboxExternalID(id *oapi.ExternalID) *string {
	if id == nil {
		return nil
	}
	return &id.ExternalId
}

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
