import { createController } from '../ProviderService'
import { ProviderControllers } from '../Provider'
import { Email } from './Email'
import EmailProvider from './EmailProvider'
import { uuid } from '../../utilities'
import App from '../../app'
import { sendMessage } from '../external/webhook'
import { logger } from '../../config/logger'

export default class WebhookEmailProvider extends EmailProvider {
    async send(message: Email): Promise<any> {
        if (!App.main.env.webhooks.send) {
            logger.warn('Webhook sending is disabled, not sending email')
            return
        }

        if (!this.external_id) {
            throw new Error('No external ID set for webhook email provider')
        }

        await sendMessage(this.external_id, {
            type: 'email' as const,
            from: typeof message.from === 'string'
                ? message.from
                : { name: message.from.name, email: message.from.address },
            to: message.to,
            subject: message.subject,
            text: message.text,
            html: message.html,
            headers: message.headers,
        })

        return {
            messageId: uuid(),
            messageSize: 0,
            messageTime: Date.now(),
            envelope: {},
            accepted: [message.to],
            rejected: [],
            pending: [],
            response: 'Message sent to external',
        }
    }

    static controllers(): ProviderControllers {
        return { admin: createController('webhook', this) }
    }
}
