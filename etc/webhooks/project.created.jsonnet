// Default project.created body. Reproduces the shape documented in
// oapi/webhooks.yml, which is what a hook with no `body` sends.
function(ctx) {
  event: ctx.event,
  timestamp: ctx.occurred_at,
  project: ctx.payload.project,

  // Who triggered the event. Use this instead of relying on a forwarded
  // Authorization header.
  triggered_by: {
    type: ctx.actor.type,
    id: ctx.actor.id,
    organization_id: ctx.actor.organization_id,
  },
}
