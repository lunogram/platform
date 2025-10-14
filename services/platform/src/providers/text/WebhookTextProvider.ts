import { sendMessage } from '../external/webhook'
import { ProviderControllers } from '../Provider'
import { createController } from '../ProviderService'
import { InboundTextMessage, TextMessage, TextResponse } from './TextMessage'
import { TextProvider } from './TextProvider'
import App from '../../app'
import { logger } from '../../config/logger'

export default class WebhookTextProvider extends TextProvider {
    async send(message: TextMessage): Promise<TextResponse> {
        if (!App.main.env.webhooks.send) {
            logger.warn('Webhook sending is disabled, not sending text message')
            return { success: false, response: 'Webhook sending is disabled', message }
        }

        if (!this.external_id) {
            throw new Error('No external ID set for webhook text provider')
        }

        await sendMessage(this.project_id, {
            type: 'text' as const,
            to: message.to,
            text: message.text,
        })

        return {
            message,
            success: true,
            response: '',
        }
    }

    parseInbound(inbound: any): InboundTextMessage {
        return {
            to: inbound.to,
            from: inbound.from,
            text: inbound.text,
        }
    }

    static controllers(): ProviderControllers {
        return { admin: createController('text', this) }
    }
}
