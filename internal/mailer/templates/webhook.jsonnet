// The default body for the webhook channel.
//
// It produces the shape Mailpit's send API accepts, which is what the
// docker-compose quickstart points at. A deployment sending through its own
// service or an HTTP provider overrides mail.webhook.body with a template
// producing that provider's shape; nothing else has to change.
function(ctx) {
  From: {
    Email: ctx.from.address,
    Name: ctx.from.name,
  },
  To: [{ Email: ctx.message.to }],
  Subject: ctx.message.subject,
  HTML: ctx.message.html,
  Text: ctx.message.text,
}
