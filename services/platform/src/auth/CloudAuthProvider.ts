import { AuthTypeConfig } from './Auth'
import { RequestError } from '../core/errors'
import AuthError from './AuthError'
import { jwtVerify, retrieveAuthToken } from './AuthMiddleware'
import AuthProvider, { AuthContext } from './AuthProvider'
import { createOrUpdateAdmin, deleteAdmin, getAdminByExternalId } from './AdminRepository'
import { createOrganization } from '../organizations/OrganizationService'
import { createClerkClient, ClerkClient, WebhookEvent } from '@clerk/backend'
import { logger } from '../config/logger'
import { Webhook } from 'svix'

export interface CloudConfig extends AuthTypeConfig {
    driver: 'cloud'
    secretKey: string
    webhookSecret: string
}

export default class CloudAuthProvider extends AuthProvider {
    private client: ClerkClient
    private webhookClient?: Webhook

    constructor(config: CloudConfig) {
        super()

        this.webhookClient = config.webhookSecret ? new Webhook(config.webhookSecret) : undefined
        this.client = createClerkClient({
            secretKey: config.secretKey,
        })
    }

    async start(): Promise<void> {
        throw new Error('Method not implemented.')
    }

    async validate(ctx: AuthContext): Promise<void> {
        logger.trace('Validating cloud Auth...')

        const token = retrieveAuthToken(ctx)
        if (!token) {
            logger.error('No token provided')
            throw new RequestError(AuthError.InvalidToken)
        }

        const payload = await jwtVerify(token)
        if (!payload || !payload.sub) {
            logger.error('Invalid JWT payload')
            throw new RequestError(AuthError.InvalidToken)
        }

        const admin = await getAdminByExternalId(payload.sub)
        if (!admin) {
            const user = await this.client.users.getUser(payload.sub)
            if (!user) {
                logger.error(`Clerk user not found: ${payload.sub}`)
                throw new RequestError(AuthError.InvalidToken)
            }

            const primaryEmailAddress = user.primaryEmailAddress
            if (!primaryEmailAddress) {
                logger.error(`Clerk user has no email: ${payload.sub}`)
                throw new RequestError(AuthError.InvalidEmail)
            }

            const organization = await createOrganization()
            await createOrUpdateAdmin({
                email: primaryEmailAddress.emailAddress,
                external_id: payload.sub,
                organization_id: organization.id,
                role: 'admin',
            })
        }
    }

    async webhook(ctx: AuthContext) {
        if (!this.webhookClient) {
            logger.error('Webhook client not configured')
            throw new RequestError(AuthError.AccessDenied)
        }

        const svixId = ctx.req.headers['svix-id']
        const svixTimestamp = ctx.req.headers['svix-timestamp']
        const svixSignature = ctx.req.headers['svix-signature']

        if (!svixId || !svixTimestamp || !svixSignature) {
            logger.error('Missing Svix headers')
            throw new RequestError(AuthError.AccessDenied)
        }

        const payloadString = ctx.body || ctx.request.body
        const { type, data } = this.webhookClient.verify(payloadString, {
            'svix-id': svixId as string,
            'svix-timestamp': svixTimestamp as string,
            'svix-signature': svixSignature as string,
        }) as WebhookEvent

        switch (type) {
            case 'user.created': {
                const createdExternalAdmin = await getAdminByExternalId(data.id)
                if (createdExternalAdmin) {
                    return
                }

                const primaryEmailAddress = data.email_addresses?.find((email: any) => email.id === data.primary_email_address_id)
                if (!primaryEmailAddress) {
                    logger.error(`Clerk user has no email: ${data.id}`)
                    throw new RequestError(AuthError.InvalidEmail)
                }

                const organization = await createOrganization()
                await createOrUpdateAdmin({
                    email: primaryEmailAddress?.email_address,
                    external_id: data.id,
                    organization_id: organization.id,
                    role: 'admin',
                })
                break
            }
            case 'user.updated': {
                const updatedExternalAdmin = await getAdminByExternalId(data.id)
                if (!updatedExternalAdmin) {
                    return
                }

                const primaryEmailAddress = data.email_addresses?.find((email) => email.id === data.primary_email_address_id)
                if (!primaryEmailAddress) {
                    logger.error(`Clerk user has no email: ${data.id}`)
                    throw new RequestError(AuthError.InvalidEmail)
                }

                updatedExternalAdmin.email = primaryEmailAddress.email_address
                await createOrUpdateAdmin(updatedExternalAdmin)
                break
            }
            case 'user.deleted': {
                if (!data.id) {
                    return
                }

                const deletedExternalAdmin = await getAdminByExternalId(data.id)
                if (!deletedExternalAdmin) {
                    return
                }

                await deleteAdmin(deletedExternalAdmin.id)
                break
            }
        }
    }
}
