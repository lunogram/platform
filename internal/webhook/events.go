package webhook

import (
	"github.com/lunogram/platform/internal/pubsub/schemas"
	"github.com/lunogram/platform/oapi"
)

// ProjectCreated fires after a project and its RBAC tuples are provisioned.
//
// The name is taken from [schemas.EventProjectCreated] rather than repeated as
// a literal, because the same event is also published to the JetStream
// projects stream by the create-project handler. Those are two delivery paths
// for one event, not two events: binding both to the same constant is what
// stops them drifting into two subtly different names that operators would
// have to know apart.
//
// The two paths differ in what they can offer. Hook delivery is synchronous and
// in-request, which is the only way can_interrupt can abort the originating
// operation and the only place an inbound request's headers still exist. The
// JetStream path is durable and redelivering, which is what best-effort
// (can_interrupt: false) hooks want, and it has no consumer today — see
// docs/content/docs/settings/webhooks.mdx for how the two are sequenced.
var ProjectCreated = MustRegister(Definition{
	Name:    schemas.EventProjectCreated,
	Version: "v1",
})

// ProjectCreatedPayload is the `ctx.payload` of a [ProjectCreated] event.
//
// The project shape stays the generated [oapi.ProjectDetails] because
// oapi/webhooks.yml is the published contract with external receivers, and a
// hand-rolled struct alongside it would be a second contract free to drift.
// What the engine no longer does is freeze that shape into the wire format: the
// embedded default template reproduces it exactly, and an operator who needs a
// different body writes a template rather than waiting on a schema change.
type ProjectCreatedPayload struct {
	Project oapi.ProjectDetails `json:"project"`
}
