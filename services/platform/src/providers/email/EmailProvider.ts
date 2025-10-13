import nodemailer from 'nodemailer'
import { LoggerProviderName } from '../LoggerProvider'
import Provider, { ExternalProviderParams, ProviderGroup, ProviderSchema } from '../Provider'
import { Email } from './Email'
import { RateLimitEmailError } from './EmailError'
import { logger } from '../../config/logger'
import { JSONSchemaType } from 'ajv'

export type EmailProviderName = 'ses' | 'smtp' | 'mailgun' | 'sendgrid' | LoggerProviderName

export interface EmailProviderParams {
    default_from?: string
    default_from_name?: string
    default_reply_to?: string
}

export function EmailProviderSchema<_ extends ExternalProviderParams, D>(
    id: string,
    data: JSONSchemaType<D>,
): any {
    const extendedData: JSONSchemaType<D & EmailProviderParams> = {
        ...data,
        properties: {
            ...data.properties,
            default_from: {
                type: 'string',
                description: 'Default "From" email address used if not overridden.',
                nullable: true,
            },
            default_from_name: {
                type: 'string',
                description: 'Default display name used in emails.',
                nullable: true,
            },
            default_reply_to: {
                type: 'string',
                description: 'Default reply-to email address.',
                nullable: true,
            },
        },
    } as JSONSchemaType<D & EmailProviderParams>

    return ProviderSchema(id, extendedData)
}

export default abstract class EmailProvider extends Provider {
    unsubscribe?: string
    transport?: nodemailer.Transporter
    boot?(): void

    static group = 'email' as ProviderGroup

    async send(message: Email): Promise<any> {
        try {
            logger.debug({ provider: this.name, to: message.to, subject: message.subject }, 'sending email')
            return await this.transport?.sendMail(message)
        } catch (error: any) {
            const isThrottle = error.code === 'Throttling'
                || error.name === 'ThrottlingException'
                || (error.message && error.message.includes('Throttling'))
                || (error.cause && error.cause.name === 'ThrottlingException')
            if (isThrottle) throw new RateLimitEmailError(error.message)
            throw error
        }
    }

    async verify(): Promise<boolean> {
        await this.transport?.verify()
        return true
    }
}
