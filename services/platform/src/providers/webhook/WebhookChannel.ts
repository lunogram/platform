import { WebhookTemplate } from '../../render/Template'
import { Variables } from '../../render'
import { WebhookProvider } from './WebhookProvider'
import { WebhookResponse } from './Webhook'
import App from '../../app'
import { cacheGet, cacheSet } from '../../config/redis'

export default class WebhookChannel {
    readonly provider: WebhookProvider
    constructor(provider?: WebhookProvider) {
        if (provider) {
            this.provider = provider
        } else {
            throw new Error('A valid webhook driver must be defined!')
        }
    }

    async send(template: WebhookTemplate, variables: Variables): Promise<WebhookResponse> {

        const message = template.compile(variables)
        const redis = App.main.redis

        // If we have a cache key, check cache first
        if (message.cacheKey?.length) {
            const key = `wh:${variables.context.campaign_id}:${message.cacheKey}`
            const value = await cacheGet<WebhookResponse>(redis, key)
            if (value) return value
            const response = await this.provider.send(message)

            await cacheSet(redis, key, response, 3600)
            return response
        }

        return await this.provider.send(message)
    }
}
