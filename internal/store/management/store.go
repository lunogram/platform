package management

import (
	"github.com/lunogram/platform/internal/store"
)

func NewState(db store.DB) *State {
	return &State{
		AdminsStore:           NewAdminsStore(db),
		ProjectsStore:         NewProjectsStore(db),
		CampaignsStore:        NewCampaignsStore(db),
		ProvidersStore:        NewProvidersStore(db),
		TemplatesStore:        NewTemplatesStore(db),
		SubscriptionsStore:    NewSubscriptionsStore(db),
		OrganizationsStore:    NewOrganizationsStore(db),
		TagsStore:             NewTagsStore(db),
		LocalesStore:          NewLocalesStore(db),
		DocumentsStore:        NewDocumentsStore(db),
		AuthStore:             NewAuthStore(db),
		ApiKeysStore:          NewApiKeysStore(db),
		ActionsStore:          NewActionsStore(db),
		SenderIdentitiesStore: NewSenderIdentitiesStore(db),
	}
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
}
