import Router from '@koa/router'
import { JSONSchemaType, validate } from '../core/validate'
import { Context } from 'koa'
import { JwtAdmin } from '../auth/AuthMiddleware'
import { deleteOrganization, getOrganization, organizationIntegrations, organizationRoleMiddleware, requireOrganizationRole, updateOrganization } from './OrganizationService'
import Organization, { OrganizationParams } from './Organization'

const router = new Router<{
    admin: JwtAdmin
    organization: Organization
}>({
    prefix: '/organizations',
})

router.use(async (ctx: Context, next: () => void) => {
    ctx.state.organization = await getOrganization(ctx.state.admin.organization_id)
    return next()
})

router.get('/', async ctx => {
    ctx.body = ctx.state.organization
})

router.use(organizationRoleMiddleware('admin'))

router.get('/integrations', async ctx => {
    ctx.body = await organizationIntegrations(ctx.state.organization)
})

const organizationUpdateParams: JSONSchemaType<OrganizationParams> = {
    $id: 'organizationUpdate',
    type: 'object',
    required: ['username'],
    properties: {
        username: { type: 'string' },
        domain: {
            type: 'string',
            nullable: true,
        },
        tracking_deeplink_mirror_url: {
            type: 'string',
            nullable: true,
        },
    },
    additionalProperties: false,
}
router.patch('/:id', async ctx => {
    requireOrganizationRole(ctx.state.admin!, 'owner')
    const payload = validate(organizationUpdateParams, ctx.request.body)
    ctx.body = await updateOrganization(ctx.state.organization, payload)
})

router.delete('/', async ctx => {
    requireOrganizationRole(ctx.state.admin!, 'owner')
    await deleteOrganization(ctx.state.organization)
    ctx.body = true
})

export default router
