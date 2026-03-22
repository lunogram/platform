package subjects

import (
	"github.com/lunogram/platform/internal/store"
	"go.uber.org/zap"
)

func NewState(db store.DB, logger *zap.Logger) *State {
	return &State{
		UsersStore:         NewUsersStore(db),
		OrganizationsStore: NewOrganizationsStore(db),
		EventsStore:        NewEventsStore(db),
		DevicesStore:       NewDevicesStore(db),
		ListsStore:         NewListsStore(db),
		RulesStore:         NewRulesStore(db),
		ActionsStore:       NewActionsStore(db),
		CampaignSendsStore: NewCampaignSendsStore(db),
		ScheduledStore:     NewScheduledStore(db, logger),
	}
}

type State struct {
	*UsersStore
	*OrganizationsStore
	*EventsStore
	*DevicesStore
	*ListsStore
	*RulesStore
	*ActionsStore
	*CampaignSendsStore
	*ScheduledStore
}
