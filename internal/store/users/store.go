package users

import (
	"github.com/lunogram/platform/internal/store"
)

func NewState(db store.DB) *State {
	return &State{
		UsersStore:   NewUsersStore(db),
		EventsStore:  NewEventsStore(db),
		DevicesStore: NewDevicesStore(db),
		ListsStore:   NewListsStore(db),
		RulesStore:   NewRulesStore(db),
	}
}

type State struct {
	*UsersStore
	*EventsStore
	*DevicesStore
	*ListsStore
	*RulesStore
}
