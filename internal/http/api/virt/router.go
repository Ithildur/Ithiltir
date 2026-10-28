package virt

import (
	virtid "dash/internal/http/api/virt/id"
	"dash/internal/nodesession"
	"dash/internal/store"
	"github.com/Ithildur/EiluneKit/http/routes"
)

func Router(st *store.Stores, sessions *nodesession.Hub) *routes.Blueprint {
	r := routes.NewBlueprint()
	r.Include("/{id}", virtid.Router(st, sessions))
	return r
}
