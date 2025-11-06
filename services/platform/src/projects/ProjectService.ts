import { JwtAdmin, ProjectState } from '../auth/AuthMiddleware'
import { Next, ParameterizedContext } from 'koa'
import { RequestError } from '../core/errors'
import { PageParams } from '../core/searchParams'
import { createSubscription } from '../subscriptions/SubscriptionService'
import { uuid } from '../utilities'
import Project, { ProjectParams, ProjectRole, projectRoles } from './Project'
import { ProjectAdmin } from './ProjectAdmins'
import { ProjectApiKey, ProjectApiKeyParams } from './ProjectApiKey'
import { getAdmin } from '../auth/AdminRepository'
import Locale, { LocaleParams } from './Locale'
import { UUID } from 'crypto'
import { createProvider } from '../providers/ProviderRepository'
import App from '../app'
import { BootstrapResponse } from '../../oapi/webhooks'
import { ProviderGroup, RateInterval } from '../providers/Provider'
import { logger } from '../config/logger'

export const adminProjectIds = async (adminId: UUID) => {
    const records = await ProjectAdmin.all(qb => qb.where('admin_id', adminId))
    return records.map(item => item.project_id)
}

export const pagedProjects = async (params: PageParams, adminId: UUID, organizationId: UUID) => {
    const admin = await getAdmin(adminId, organizationId)
    const projectIds = await adminProjectIds(adminId)
    return await Project.search({ ...params, fields: ['name'] }, qb =>
        qb.where(qb =>
            qb.where('organization_id', admin!.organization_id)
                .orWhereIn('projects.id', projectIds),
        ),
    )
}

export const allProjects = async (adminId: UUID, organizationId: UUID) => {
    const admin = await getAdmin(adminId, organizationId)
    if (!admin) return []
    const projectIds = await adminProjectIds(adminId)
    return await Project.all(qb => {
        qb.whereIn('projects.id', projectIds)
        if (admin.role !== 'member') {
            qb.orWhere('organization_id', admin.organization_id)
        }
        return qb
    })
}

export const getProject = async (id: UUID, adminId?: UUID) => {
    const project = await Project.first(
        qb => {
            qb.where('projects.id', id).select('projects.*')
                .select(
                    Project.raw(
                        '(SELECT COUNT(*) FROM campaigns WHERE campaigns.project_id = projects.id) AS campaigns_count',
                    ),
                    Project.raw(
                        '(SELECT COUNT(*) FROM journeys WHERE journeys.project_id = projects.id) AS journeys_count',
                    ),
                    Project.raw(
                        '(SELECT COUNT(*) FROM users WHERE users.project_id = projects.id) AS users_count',
                    ),
                    Project.raw(
                        '(SELECT COUNT(*) FROM lists WHERE lists.project_id = projects.id) AS lists_count',
                    ),
                )
            if (adminId != null) {
                qb.leftJoin('project_admins', 'project_admins.project_id', 'projects.id')
                    .where('admin_id', adminId)
                    .select('role')
            }
            return qb
        })

    if (!project) {
        return null
    }

    return {
        ...project,
        campaigns_count: Number(project?.campaigns_count ?? 0),
        journeys_count: Number(project?.journeys_count ?? 0),
        users_count: Number(project?.users_count ?? 0),
        lists_count: Number(project?.lists_count ?? 0),
    } as Project
}

export const createProject = async (admin: JwtAdmin, params: ProjectParams): Promise<Project> => {
    const projectId = await Project.insert({
        ...params,
        organization_id: admin.organization_id,
    })

    // Add the user creating the project to it
    await ProjectAdmin.insert({
        project_id: projectId,
        admin_id: admin.id,
        role: 'admin',
    })

    // Create initial locale for project
    const languages = new Intl.DisplayNames([params.locale], { type: 'language' })
    await Locale.insert({
        project_id: projectId,
        key: params.locale,
        label: languages.of(params.locale),
    })

    // Create a single subscription for each type
    await createSubscription(projectId, { name: 'Default Email', channel: 'email', is_public: true })
    await createSubscription(projectId, { name: 'Default SMS', channel: 'text', is_public: true })
    await createSubscription(projectId, { name: 'Default Push', channel: 'push', is_public: true })
    await createSubscription(projectId, { name: 'Default Webhook', channel: 'webhook', is_public: false })

    const project = await getProject(projectId, admin.id)
    await bootstrapProject(project!)

    return project!
}

export const bootstrapProject = async (project: Project): Promise<void> => {
    if (!App.main.env.webhooks.bootstrap) {
        return
    }

    logger.info({ projectId: project.id }, 'Bootstrapping project')
    const { providers } = await fetchProvidersBootstrapConfig(project.organization_id, project.id)
    if (!providers || providers.length === 0) {
        logger.info({ projectId: project.id }, 'No providers to bootstrap')
        return
    }

    for (const provider of providers) {
        logger.info({ projectId: project.id, type: provider.type }, 'Setting up provider')
        await createProvider(project.id, {
            type: provider.type,
            name: provider.name,
            data: provider.data || {},
            group: provider.group as ProviderGroup,
            rate_interval: provider.rate_interval as RateInterval,
            rate_limit: Number(provider.rate_limit ?? 0),
            is_default: provider.is_default || false,
            external_id: provider.external_id ?? undefined,
        })
    }

}

export async function fetchProvidersBootstrapConfig(
    organizationId: UUID,
    projectId: UUID,
): Promise<BootstrapResponse> {
    if (!App.main.env.webhooks.bootstrap) {
        throw new Error('No bootstrap webhook configured')
    }

    const controller = new AbortController()
    const timeoutId = setTimeout(() => controller.abort(), App.main.env.webhooks.bootstrap.timeoutMs)

    try {
        const res = await fetch(App.main.env.webhooks.bootstrap.url, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({
                project_id: projectId,
                organization_id: organizationId,
            }),
            signal: controller.signal,
        })

        clearTimeout(timeoutId)
        if (!res.ok) throw new Error(`Webhook error: ${res.statusText}`)
        return res.json()
    } catch (error) {
        clearTimeout(timeoutId)
        throw error
    }
}

export const updateProject = async (id: UUID, adminId: UUID, params: Partial<ProjectParams>) => {
    await Project.update(qb => qb.where('id', id), params)
    return await getProject(id, adminId)
}

export const pagedApiKeys = async (params: PageParams, projectId: UUID) => {
    return await ProjectApiKey.search(
        { ...params, fields: ['name', 'description'] },
        qb => qb.where('project_id', projectId).whereNull('deleted_at'),
    )
}

export const getProjectApiKey = async (key: string) => {
    return ProjectApiKey.first(qb => qb.where('value', key).whereNull('deleted_at'))
}

export const createProjectApiKey = async (projectId: UUID, params: ProjectApiKeyParams) => {
    return await ProjectApiKey.insertAndFetch({
        ...params,
        value: generateApiKey(params.scope),
        project_id: projectId,
    })
}

export const updateProjectApiKey = async (id: UUID, params: ProjectApiKeyParams) => {
    return await ProjectApiKey.updateAndFetch(id, params)
}

export const revokeProjectApiKey = async (id: UUID) => {
    return await ProjectApiKey.archive(id)
}

export const generateApiKey = (scope: 'public' | 'secret') => {
    const key = uuid().replace('-', '')
    const prefix = scope === 'public' ? 'pk' : 'sk'
    return `${prefix}_${key}`
}

export const requireProjectRole = (ctx: ParameterizedContext<ProjectState>, minRole: ProjectRole) => {
    if (projectRoles.indexOf(minRole) > projectRoles.indexOf(ctx.state.projectRole)) {
        throw new RequestError(`Minimum project role ${minRole} is required`, 403)
    }
}

export const projectRoleMiddleware = (minRole: ProjectRole) => async (ctx: ParameterizedContext<ProjectState>, next: Next) => {
    requireProjectRole(ctx, minRole)
    return next()
}

export const pagedLocales = async (params: PageParams, projectId: UUID) => {
    return await Locale.search(
        { ...params, fields: ['label'] },
        qb => qb.where('project_id', projectId),
    )
}

export const createLocale = async (projectId: UUID, params: LocaleParams) => {
    return await Locale.insertAndFetch({
        ...params,
        project_id: projectId,
    })
}

export const getLocale = async (projectId: UUID, key: string) => {
    return await Locale.first(qb =>
        qb.where('project_id', projectId)
            .where('key', key),
    )
}

export const deleteLocale = async (projectId: UUID, id: UUID) => {
    return await Locale.deleteById(id, qb => qb.where('project_id', projectId))
}
