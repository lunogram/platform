import Router from '@koa/router'
import { JSONSchemaType, validate } from '../core/validate'
import Campaign, { CampaignCreateParams, CampaignUpdateParams } from './Campaign'
import { archiveCampaign, createCampaign, deleteCampaign, duplicateCampaign, getCampaign, getCampaignUsers, pagedCampaigns, updateCampaign } from './CampaignService'
import { searchParamsSchema, SearchSchema } from '../core/searchParams'
import { extractQueryParams } from '../utilities'
import { ProjectState } from '../auth/AuthMiddleware'
import { projectRoleMiddleware } from '../projects/ProjectService'
import { Context, Next } from 'koa'
import { validate as uuidValidate } from 'uuid'
import { UUID } from 'crypto'

const router = new Router<ProjectState & { campaign?: Campaign }>({
    prefix: '/campaigns',
})

const checkCampaignId = async (value: string, ctx: Context, next: Next) => {
    if (!uuidValidate(value)) {
        ctx.throw(400, 'Invalid campaign ID')
        return
    }

    ctx.state.campaign = await getCampaign(value as UUID, ctx.state.project.id)
    if (!ctx.state.campaign) {
        ctx.throw(404)
        return
    }
    return await next()
}

router.use(projectRoleMiddleware('editor'))

router.get('/', async ctx => {
    const searchSchema = SearchSchema('campaignSearchSchema', {
        sort: 'created_at',
        direction: 'desc',
    })
    const params = extractQueryParams(ctx.query, searchSchema)
    ctx.body = await pagedCampaigns(params, ctx.state.project.id)
})

const campaignCreateParams: JSONSchemaType<CampaignCreateParams> = {
    $id: 'campaignCreate',
    type: 'object',
    required: ['type', 'subscription_id', 'provider_id'],
    additionalProperties: false,
    properties: {
        type: {
            type: 'string',
            enum: ['blast', 'trigger'],
        },
        name: {
            type: 'string',
            nullable: true,
        },
        channel: {
            type: 'string',
            enum: ['email', 'text', 'push', 'webhook', 'in_app'],
            nullable: true,
        },
        subscription_id: {
            type: 'string',
            format: 'uuid',
        },
        provider_id: {
            type: 'string',
            format: 'uuid',
        },
        list_ids: {
            type: 'array',
            items: { type: 'string', format: 'uuid' },
            nullable: true,
        },
        exclusion_list_ids: {
            type: 'array',
            items: { type: 'string', format: 'uuid' },
            nullable: true,
        },
        send_in_user_timezone: {
            type: 'boolean',
            nullable: true,
        },
        send_at: {
            type: 'string',
            format: 'date-time',
            nullable: true,
        },
        tags: {
            type: 'array',
            items: { type: 'string' },
            nullable: true,
        },
    },
}

router.post('/', async ctx => {
    const payload = validate(campaignCreateParams, ctx.request.body)
    ctx.body = await createCampaign(ctx.state.project.id, {
        ...payload,
        admin_id: ctx.state.admin?.id,
    })
})

router.param('campaignId', checkCampaignId)

router.get('/:campaignId', async ctx => {
    ctx.body = ctx.state.campaign!
})

const campaignUpdateParams: JSONSchemaType<Partial<CampaignUpdateParams>> = {
    $id: 'campaignUpdate',
    type: 'object',
    required: [],
    properties: {
        name: {
            type: 'string',
            nullable: true,
        },
        subscription_id: {
            type: 'string',
            format: 'uuid',
            nullable: true,
        },
        provider_id: {
            type: 'string',
            format: 'uuid',
            nullable: true,
        },
        state: {
            type: 'string',
            enum: ['draft', 'scheduled', 'finished', 'aborted'],
            nullable: true,
        },
        list_ids: {
            type: 'array',
            items: { type: 'string', format: 'uuid' },
            nullable: true,
        },
        exclusion_list_ids: {
            type: 'array',
            items: { type: 'string', format: 'uuid' },
            nullable: true,
        },
        send_in_user_timezone: {
            type: 'boolean',
            nullable: true,
        },
        send_at: {
            type: 'string',
            format: 'date-time',
            nullable: true,
        },
        tags: {
            type: 'array',
            items: {
                type: 'string',
            },
            nullable: true,
        },
    },
    additionalProperties: false,
}

router.patch('/:campaignId', async ctx => {
    const payload = validate(campaignUpdateParams, ctx.request.body)
    ctx.body = await updateCampaign(ctx.state.campaign!.id, ctx.state.project.id, {
        ...payload,
        admin_id: ctx.state.admin?.id,
    })
})

router.get('/:campaignId/users', async ctx => {
    const params = extractQueryParams(ctx.query, searchParamsSchema)
    ctx.body = await getCampaignUsers(ctx.state.campaign!.id, params, ctx.state.project.id)
})

router.delete('/:campaignId', async ctx => {
    const campaign = ctx.state.campaign!
    if (campaign.deleted_at) {
        await deleteCampaign(campaign, ctx.state.admin?.id)
    } else {
        await archiveCampaign(campaign, ctx.state.admin?.id)
    }
    ctx.body = true
})

router.post('/:campaignId/duplicate', async ctx => {
    ctx.body = await duplicateCampaign(ctx.state.campaign!, ctx.state.admin?.id)
})

export default router
