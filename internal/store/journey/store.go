package journey

import (
	"github.com/lunogram/platform/internal/store"
)

func NewState(db store.DB) *State {
	return &State{
		JourneysStore: NewJourneysStore(db),
	}
}

type State struct {
	*JourneysStore
}
