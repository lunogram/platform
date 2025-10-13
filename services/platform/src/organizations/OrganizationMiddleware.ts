import { Context } from 'koa'
import { getOrganization } from './OrganizationService'
import { UUID } from 'crypto'
import { validate as uuidValidate } from 'uuid'

export const organizationMiddleware = async (ctx: Context, next: () => void) => {
    const organizationId = ctx.cookies.get('organization', { signed: true }) as UUID | undefined
    if (!organizationId) {
        return next()
    }

    if (!uuidValidate(organizationId)) {
        ctx.throw(400, 'Invalid organization ID')
        return
    }

    ctx.state.organization = await getOrganization(organizationId)
    return next()
}
