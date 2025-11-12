import PushJob from '../providers/push/PushJob'
import WebhookJob from '../providers/webhook/WebhookJob'
import TextJob from '../providers/text/TextJob'
import EmailJob from '../providers/email/EmailJob'
import { logger } from '../config/logger'
import { User } from '../users/User'
import Campaign, { CampaignCreateParams, CampaignDelivery, CampaignParams, CampaignPopulationProgress, CampaignProgress, CampaignSend, CampaignSendReferenceType, CampaignSendState, CampaignState, SentCampaign } from './Campaign'
import List from '../lists/List'
import Subscription, { SubscriptionState } from '../subscriptions/Subscription'
import { RequestError } from '../core/errors'
import { PageParams } from '../core/searchParams'
import { allLists } from '../lists/ListService'
import { allTemplates, createTemplate, duplicateTemplate, validateTemplates } from '../render/TemplateService'
import { getSubscription, getUserSubscriptionState } from '../subscriptions/SubscriptionService'
import { chunk, pick, shallowEqual } from '../utilities'
import { getProvider, getDefaultProvider } from '../providers/ProviderRepository'
import { createTagSubquery, getTags, setTags } from '../tags/TagService'
import { getProject } from '../projects/ProjectService'
import CampaignError from './CampaignError'
import CampaignGenerateListJob from './CampaignGenerateListJob'
import { differenceInDays, subDays } from 'date-fns'
import { cacheDel, cacheGet } from '../config/redis'
import App from '../app'
import CampaignAbortJob from './CampaignAbortJob'
import { getJourneysForCampaign } from '../journey/JourneyService'
import { createAuditLog } from '../core/audit/AuditService'
import { WithAdmin } from '../core/audit/Audit'
import { UUID } from 'node:crypto'
import Provider from '../providers/Provider'

export const CacheKeys = {
    pendingStats: 'campaigns:pending_stats',
    generate: (campaign: Campaign) => `campaigns:${campaign.id}:generate:users`,
    generateReady: (campaign: Campaign) => `campaigns:${campaign.id}:generate:ready`,
    populationProgress: (campaign: Campaign) => `campaigns:${campaign.id}:progress`,
    populationTotal: (campaign: Campaign) => `campaigns:${campaign.id}:total`,
}

export const pagedCampaigns = async (params: PageParams, projectId: UUID) => {
    const result = await Campaign.search(
        { ...params, fields: ['name'] },
        b => {
            b.where('project_id', projectId)
                .whereNull('deleted_at')
            if (params.filter?.type) {
                b.where('type', params.filter.type)
            }
            params.tag?.length && b.whereIn('id', createTagSubquery(Campaign, projectId, params.tag))
            return b
        },
    )
    if (result.results?.length) {
        const tags = await getTags(Campaign.tableName, result.results.map(c => c.id))
        for (const campaign of result.results) {
            campaign.tags = tags.get(campaign.id)
        }
    }

    return result
}

export const allCampaigns = async (projectId: UUID): Promise<Campaign[]> => {
    return await Campaign.all(qb => qb.where('project_id', projectId))
}

export const getCampaign = async (id: UUID, projectId: UUID): Promise<Campaign | undefined> => {
    const campaign = await Campaign.find(id,
        qb => qb.where('project_id', projectId)
            .whereNull('deleted_at'),
    )

    if (!campaign) return

    campaign.provider = await getProvider(campaign.provider_id, projectId)
    campaign.templates = await allTemplates(projectId, campaign.id)
    campaign.lists = campaign.list_ids ? await allLists(projectId, campaign.list_ids) : []
    campaign.exclusion_lists = campaign.exclusion_list_ids ? await allLists(projectId, campaign.exclusion_list_ids) : []
    campaign.subscription = await getSubscription(campaign.subscription_id, projectId)
    campaign.tags = await getTags(Campaign.tableName, [campaign.id]).then(m => m.get(campaign.id))

    if (campaign.type === 'trigger') campaign.journeys = await getJourneysForCampaign(projectId, campaign.id)
    if (campaign.state === 'loading') {
        campaign.progress = await campaignPopulationProgress(campaign)
    }

    return campaign
}

export const getCampaignProvider = async (id: UUID, projectId: UUID): Promise<Provider | undefined> => {
    const campaign = await Campaign.find(id,
        qb => qb.where('project_id', projectId)
            .whereNull('deleted_at'),
    )

    if (!campaign) return
    return await getProvider(campaign.provider_id, projectId)
}

export const createCampaign = async (projectId: UUID, { tags, admin_id, ...params }: WithAdmin<CampaignCreateParams>): Promise<Campaign> => {
    const project = await getProject(projectId)
    if (!project) {
        throw new RequestError(CampaignError.CampaignProjectNotFound)
    }

    if (!params.provider_id) {
        const defaultProvider = await getDefaultProvider(projectId, params.channel)
        if (defaultProvider) {
            params.provider_id = defaultProvider.id
        }
    }

    const delivery = { sent: 0, total: 0, opens: 0, clicks: 0 }
    const campaign = await Campaign.insertAndFetch({
        ...params,
        state: 'draft',
        delivery,
        channel: params.channel,
        project_id: projectId,
    })

    if (tags?.length) {
        await setTags({
            project_id: projectId,
            entity: Campaign.tableName,
            entity_id: campaign.id,
            names: tags,
        })
    }

    if (admin_id) {
        await createAuditLog({
            project_id: projectId,
            admin_id,
            event: 'create',
            object: campaign,
        })
    }

    // NOTE: we always create an initial template for the campaign in the project locale
    await createTemplate(projectId, {
        campaign_id: campaign.id,
        locale: project.locale,
        type: params.channel,
        data: {},
    })

    return await getCampaign(campaign.id, projectId) as Campaign
}

export const updateCampaign = async (id: UUID, projectId: UUID, { tags, admin_id, ...params }: WithAdmin<Partial<CampaignParams>>): Promise<Campaign | undefined> => {

    // Ensure finished campaigns are no longer modified
    const campaign = await getCampaign(id, projectId) as Campaign
    if (campaign.state === 'finished') {
        throw new RequestError(CampaignError.CampaignFinished)
    }

    // Check that provider is valid
    if (params.provider_id) {
        const provider = await getProvider(params.provider_id, projectId)
        if (provider?.deleted_at) throw new RequestError(CampaignError.CampaignInvalidProvider)
    }

    const data: Partial<Campaign> = { ...params }
    let send_at: Date | undefined | null = data.send_at ? new Date(data.send_at) : undefined

    const isRescheduling = send_at != null
        && campaign.send_at != null
        && send_at !== campaign.send_at

    // If we are aborting, reset `send_at`
    if (data.state === 'aborted') {
        send_at = null
        data.state = 'aborting'
    }

    // If we are rescheduling, abort sends so they are reset
    if (isRescheduling) {
        data.state = 'aborting'
    }

    // Check templates to make sure we can schedule a send
    if (data.state === 'scheduled') {
        await validateTemplates(projectId, id)

        // Set to loading if success so scheduling starts
        data.state = 'loading'
    }

    // If this is a trigger campaign, should always be running
    if (data.type === 'trigger') {
        data.state = 'running'
    }

    const newCampaign = await Campaign.updateAndFetch(id, {
        ...data,
        send_at,
    })

    if (tags) {
        await setTags({
            project_id: projectId,
            entity: Campaign.tableName,
            entity_id: id,
            names: tags,
        })
    }

    if (data.state === 'loading' && campaign.type === 'blast') {
        await CampaignGenerateListJob.from(campaign).queue()
    }

    if (data.state === 'aborting') {
        await CampaignAbortJob.from({ ...campaign, reschedule: isRescheduling }).queue()
    }

    if (admin_id) {
        const event = data.state === 'aborting'
            ? 'aborted'
            : data.state === 'loading'
                ? 'launched'
                : 'updated'
        await createAuditLog({
            project_id: projectId,
            admin_id,
            event,
            object: newCampaign,
            previous: campaign,
        })
    }

    return await getCampaign(id, projectId)
}

export const archiveCampaign = async (campaign: Campaign, adminId?: UUID) => {
    await Campaign.archive(campaign.id, qb => qb.where('project_id', campaign.project_id))

    if (adminId) {
        await createAuditLog({
            project_id: campaign.project_id,
            admin_id: adminId,
            event: 'archive',
            previous: campaign,
        })
    }

    return getCampaign(campaign.id, campaign.project_id)
}

export const deleteCampaign = async (campaign: Campaign, adminId?: UUID) => {
    const results = await Campaign.deleteById(campaign.id, qb => qb.where('project_id', campaign.project_id))
    if (adminId) {
        await createAuditLog({
            project_id: campaign.project_id,
            admin_id: adminId,
            event: 'delete',
            previous: campaign,
        })
    }
    return results
}

export const getCampaignUsers = async (id: UUID, params: PageParams, projectId: UUID) => {
    return await User.search(
        { ...params, fields: ['email', 'phone'], mode: 'exact' },
        b => b.rightJoin('campaign_sends', 'campaign_sends.user_id', 'users.id')
            .where('project_id', projectId)
            .where('campaign_id', id)
            .select('users.*', 'state', 'send_at', 'opened_at', 'clicks'),
    )
}

interface SendCampaign {
    campaign: Campaign
    user: User | UUID
    exists?: boolean
    reference_type?: CampaignSendReferenceType
    reference_id?: UUID
}

export const triggerCampaignSend = async ({ campaign, user, exists, reference_type, reference_id }: SendCampaign & { user: User }) => {

    // Check if the user can receive the campaign and has not unsubscribed
    if (!canSendCampaignToUser(campaign, user)) return

    const subscriptionState = await getUserSubscriptionState(user, campaign.subscription_id)
    if (subscriptionState === SubscriptionState.unsubscribed) return

    // If the send doesn't already exist, lets create it ahead of scheduling
    const reference = { reference_id, reference_type }
    if (!exists) {
        await CampaignSend.insert({
            campaign_id: campaign.id,
            user_id: user.id,
            state: 'pending',
            send_at: new Date(),
            ...reference,
        })
    }

    return sendCampaignJob({
        campaign,
        user,
        ...reference,
    })
}

export const sendCampaignJob = ({ campaign, user, reference_type, reference_id }: SendCampaign): EmailJob | TextJob | PushJob | WebhookJob => {
    const body = {
        campaign_id: campaign.id,
        user_id: user instanceof User ? user.id : user,
        reference_type,
        reference_id,
    }

    const channels = {
        email: EmailJob.from(body),
        text: TextJob.from(body),
        push: PushJob.from(body),
        webhook: WebhookJob.from(body),
    }
    const job = channels[campaign.channel]
    job.deduplicationKey(`sid_${campaign.id}_${body.user_id}_${body.reference_id}`)
    return job
}

interface UpdateSendStateParams {
    campaign: Campaign | UUID
    user: User | UUID
    state?: CampaignSendState
    reference_id?: string
    response?: any
}

export const updateSendState = async ({ campaign, user, state = 'sent', reference_id = '0' }: UpdateSendStateParams) => {
    const userId = user instanceof User ? user.id : user
    const campaignId = campaign instanceof Campaign ? campaign.id : campaign

    // Update send state
    const records = await CampaignSend.update(
        qb => qb.where('user_id', userId)
            .where('campaign_id', campaignId)
            .where('reference_id', reference_id),
        { state },
    )

    // If no records were updated then try and create missing record
    if (records <= 0) {
        const records = await CampaignSend.query()
            .insert({
                user_id: userId,
                campaign_id: campaignId,
                reference_id,
                state,
            })
            .onConflict(['campaign_id', 'user_id', 'reference_id'])
            .merge(['state'])
        return Array.isArray(records) ? records[0] : records
    }

    return records
}

const cleanupGenerationCacheKeys = async (campaign: Campaign) => {
    const redis = App.main.redis
    await cacheDel(redis, CacheKeys.generate(campaign))
    await cacheDel(redis, CacheKeys.populationTotal(campaign))
    await cacheDel(redis, CacheKeys.populationProgress(campaign))
}

export const populateSendList = async (campaign: SentCampaign) => {
    const project = await getProject(campaign.project_id)
    if (!campaign.list_ids || !project) {
        throw new RequestError('Unable to send to a campaign that does not have an associated list', 404)
    }

    throw Error('Not implemented yet')
}

export const campaignSendReadyQuery = (
    campaignId: UUID,
    includeThrottled = false,
    limit?: number,
) => {
    const query = CampaignSend.query()
        .where('campaign_sends.send_at', '<=', CampaignSend.raw('NOW()'))
        .whereIn('campaign_sends.state', includeThrottled ? ['pending', 'throttled'] : ['pending'])
        .where('campaign_id', campaignId)
        .select('user_id', 'reference_id')
    if (limit) query.limit(limit)
    return query
}

export const failStalledSends = async (campaign: Campaign) => {

    const stalledDays = 2

    // Its not possible to have any stalled records if the campaign send
    // was less than the number of days we are checking for
    if (
        campaign.send_at
        && differenceInDays(
            Date.now(),
            new Date(campaign.send_at),
        ) >= stalledDays
    ) return

    const query = CampaignSend.query()
        .where('campaign_sends.send_at', '<', subDays(Date.now(), stalledDays))
        .where('campaign_sends.state', 'throttled')
        .where('campaign_id', campaign.id)
        .select('user_id', 'campaign_id')
    await chunk(query, 25, async (items) => {
        await CampaignSend.query()
            .update({ state: 'failed' })
            .whereIn(['user_id', 'campaign_id'], items)
    }, ({ user_id, campaign_id }: CampaignSend) => ([user_id, campaign_id]))
}

export const abortCampaign = async (campaign: Campaign) => {
    await CampaignSend.query()
        .where('campaign_id', campaign.id)
        .where('state', 'pending')
        .update({ state: 'aborted' })
    await cleanupGenerationCacheKeys(campaign)
}

export const clearCampaign = async (campaign: Campaign) => {
    await CampaignSend.query()
        .where('campaign_id', campaign.id)
        .whereIn('state', ['pending', 'throttled', 'aborted'])
        .delete()
}

export const duplicateCampaign = async (campaign: Campaign, adminId?: UUID) => {
    const params: CampaignCreateParams = pick(campaign, ['project_id', 'list_ids', 'exclusion_list_ids', 'provider_id', 'subscription_id', 'channel', 'name', 'type'])
    params.name = `Copy of ${params.name}`
    const { id: cloneId } = await createCampaign(campaign.project_id, { ...params, admin_id: adminId })
    for (const template of campaign.templates) {
        await duplicateTemplate(template, cloneId)
    }

    return await getCampaign(cloneId, campaign.project_id)
}

export const campaignPopulationProgress = async (campaign: Campaign): Promise<CampaignPopulationProgress> => {
    return {
        complete: await cacheGet<number>(App.main.redis, CacheKeys.populationProgress(campaign)) ?? 0,
        total: await cacheGet<number>(App.main.redis, CacheKeys.populationTotal(campaign)) ?? 0,
    }
}

export const campaignDeliveryProgress = async (campaignId: UUID): Promise<CampaignProgress> => {
    const progress = await CampaignSend.query()
        .where('campaign_id', campaignId)
        .select(CampaignSend.raw("SUM(CASE WHEN state = 'sent' THEN 1 ELSE 0 END) AS sent, SUM(CASE WHEN state IN ('pending', 'throttled') THEN 1 ELSE 0 END) AS pending, COUNT(*) AS total, SUM(CASE WHEN opened_at IS NOT NULL THEN 1 ELSE 0 END) AS opens, SUM(CASE WHEN clicks > 0 THEN 1 ELSE 0 END) AS clicks"))
        .first()
    return {
        sent: parseInt(progress.sent ?? 0),
        pending: parseInt(progress.pending ?? 0),
        total: parseInt(progress.total ?? 0),
        opens: parseInt(progress.opens ?? 0),
        clicks: parseInt(progress.clicks ?? 0),
    }
}

export const updateCampaignProgress = async (campaign: Campaign, stateOverride?: CampaignState): Promise<void> => {
    const currentState = (pending: number, delivery: CampaignDelivery) => {
        if (campaign.type === 'trigger') return 'running'
        if (campaign.state === 'draft') return 'draft'
        if (campaign.state === 'loading') return 'loading'
        if (pending <= 0) return 'finished'
        if (delivery.sent === 0) return 'scheduled'
        return 'running'
    }

    const { pending, ...delivery } = await campaignDeliveryProgress(campaign.id)
    const state = stateOverride ?? currentState(pending, delivery)

    // If nothing has changed, continue otherwise update
    if (shallowEqual(campaign.delivery, delivery) && state === campaign.state) return

    if (state !== campaign.state) {
        await createAuditLog({
            project_id: campaign.project_id,
            event: 'state',
            object: { ...campaign, pending, delivery, state },
            previous: campaign,
        })
        logger.info({ campaignId: campaign.id, state, pending, delivery }, 'campaign:state:update')
    }

    await Campaign.update(qb => qb.where('id', campaign.id).where('project_id', campaign.project_id), { state, delivery })
}

export const getCampaignSend = async (campaignId: UUID, userId: UUID, referenceId = '0') => {
    return CampaignSend.first(qb => qb
        .where('campaign_id', campaignId)
        .where('user_id', userId)
        .where('reference_id', referenceId),
    )
}

export const updateCampaignSend = async (campaignId: UUID, userId: UUID, referenceId: string, update: Partial<CampaignSend>) => {
    await CampaignSend.update(
        qb => qb
            .where('campaign_id', campaignId)
            .where('user_id', userId)
            .where('reference_id', referenceId),
        update,
    )
}

export const estimatedSendSize = async (campaign: Campaign) => {
    const lists: List[] = await List.query().whereIn('id', campaign.list_ids ?? [])
    return lists.reduce((acc, list) => (list.users_count ?? 0) + acc, 0)
}

export const canSendCampaignToUser = (campaign: Campaign, user: Pick<User, 'email' | 'phone' | 'has_push_device' | 'devices'>) => {
    if (campaign.channel === 'email' && !user.email) return false
    if (campaign.channel === 'text' && !user.phone) return false
    if (campaign.channel === 'push' && !(user.has_push_device || !!user.devices)) return false
    return true
}
