import nodemailer from 'nodemailer'
import { LoggerProviderName } from '../LoggerProvider'
import Provider, { ProviderGroup } from '../Provider'
import { Email } from './Email'
import { RateLimitEmailError } from './EmailError'

export type EmailProviderName = 'ses' | 'smtp' | 'mailgun' | 'sendgrid' | LoggerProviderName

export default abstract class EmailProvider extends Provider {

    unsubscribe?: string
    transport?: nodemailer.Transporter
    boot?(): void

    static group = 'email' as ProviderGroup

    async send(message: Email): Promise<any> {
        try {
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
