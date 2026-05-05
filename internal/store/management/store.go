package management

import (
	"context"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/store"
)

func NewState(db store.DB) *State {
	return &State{
		AdminsStore:               NewAdminsStore(db),
		ProjectsStore:             NewProjectsStore(db),
		CampaignsStore:            NewCampaignsStore(db),
		ProvidersStore:            NewProvidersStore(db),
		TemplatesStore:            NewTemplatesStore(db),
		SubscriptionsStore:        NewSubscriptionsStore(db),
		OrganizationsStore:        NewOrganizationsStore(db),
		TagsStore:                 NewTagsStore(db),
		LocalesStore:              NewLocalesStore(db),
		DocumentsStore:            NewDocumentsStore(db),
		AuthStore:                 NewAuthStore(db),
		ApiKeysStore:              NewApiKeysStore(db),
		ActionsStore:              NewActionsStore(db),
		SenderIdentitiesStore:     NewSenderIdentitiesStore(db),
		BroadcastsStore:           NewBroadcastsStore(db),
		ProjectPushProvidersStore: NewProjectPushProvidersStore(db),
		VapidKeysStore:            NewVapidKeysStore(db),
	}
}

// ListArchivedCampaigns lists all archived (soft-deleted) campaigns
func (s *State) ListArchivedCampaigns(ctx context.Context, projectID uuid.UUID, pagination store.Pagination, search string) (Campaigns, int, error) {
	return s.CampaignsStore.ListArchivedCampaigns(ctx, projectID, pagination, search)
}

// RestoreCampaign restores a soft-deleted campaign
func (s *State) RestoreCampaign(ctx context.Context, projectID, campaignID uuid.UUID) error {
	return s.CampaignsStore.RestoreCampaign(ctx, projectID, campaignID)
}

type State struct {
	*AdminsStore
	*ProjectsStore
	*CampaignsStore
	*ProvidersStore
	*TemplatesStore
	*SubscriptionsStore
	*OrganizationsStore
	*TagsStore
	*LocalesStore
	*DocumentsStore
	*AuthStore
	*ApiKeysStore
	*ActionsStore
	*SenderIdentitiesStore
	*BroadcastsStore
	*ProjectPushProvidersStore
	*VapidKeysStore
}
