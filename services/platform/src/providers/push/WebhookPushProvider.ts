import { ProviderControllers } from '../Provider'
import { createController } from '../ProviderService'
import { Push, PushResponse } from './Push'
import { PushProvider } from './PushProvider'
import App from '../../app'
import { sendMessage } from '../external/webhook'
import { logger } from '../../config/logger'

export default class WebhookPushProvider extends PushProvider {
    async send(push: Push): Promise<PushResponse> {
        if (!App.main.env.webhooks.send) {
            logger.warn('Webhook sending is disabled, not sending push notification')
            return { success: false, response: 'Webhook sending is disabled', invalidTokens: [], count: 0, push }
        }

        if (!this.external_id) {
            throw new Error('No external ID set for webhook push provider')
        }

        await sendMessage(this.external_id, {
            type: 'push' as const,
            tokens: push.tokens,
            title: push.title,
            body: push.body,
            custom: push.custom,
        })

        return {
            push,
            success: true,
            response: '',
            invalidTokens: [],
            count: push.tokens.length,
        }
    }

    static controllers(): ProviderControllers {
        return { admin: createController('push', this) }
    }
}
