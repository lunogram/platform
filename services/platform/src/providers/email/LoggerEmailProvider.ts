import { logger } from '../../config/logger'
import { randomInt, sleep, uuid } from '../../utilities'
import { ExternalProviderParams, ProviderControllers } from '../Provider'
import { createController } from '../ProviderService'
import { Email } from './Email'
import EmailProvider, { EmailProviderSchema } from './EmailProvider'

export default class LoggerEmailProvider extends EmailProvider {
    addLatency?: boolean

    static isDevelopment = true

    static namespace = 'logger'
    static meta = {
        name: 'Logger',
        icon: 'https://lunogram.com/providers/logger.svg',
    }

    static schema = EmailProviderSchema<ExternalProviderParams, any>('loggerEmailProviderParams', {
        type: 'object',
    })

    async send(message: Email): Promise<any> {
        // Allow for having random latency to aid in performance testing
        if (this.addLatency) await sleep(randomInt())

        logger.info(message, 'provider:email:logger')

        return {
            messageId: uuid(),
            messageSize: 0,
            messageTime: Date.now(),
            envelope: {},
            accepted: [message.to],
            rejected: [],
            pending: [],
            response: 'Message sent to logger',
        }
    }

    async verify(): Promise<boolean> {
        return true
    }

    static controllers(): ProviderControllers {
        return { admin: createController('email', this) }
    }
}
