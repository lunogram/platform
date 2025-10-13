import { RequestError } from '../core/errors'
import Admin from '../auth/Admin'
import Provider from '../providers/Provider'
import Organization, { OrganizationRole, organizationRoles } from './Organization'
import { JwtAdmin } from '../auth/AuthMiddleware'
import { Next, ParameterizedContext } from 'koa'
import { UUID } from 'crypto'

export const getOrganization = async (id: UUID) => {
    return await Organization.find(id)
}

export const getOrganizationByEmail = async (email: string) => {
    const admin = await Admin.first(qb => qb.where('email', email))
    if (!admin) return undefined
    return await getOrganization(admin.organization_id)
}

export const createOrganization = async (): Promise<Organization> => {
    return await Organization.insertAndFetch()
}

export const updateOrganization = async (organization: Organization, params: Partial<Organization>) => {
    return await Organization.updateAndFetch(organization.id, params)
}

export const organizationIntegrations = async (organization: Organization) => {
    return await Provider.all(
        qb => qb.leftJoin('projects', 'projects.id', 'providers.project_id')
            .where('projects.organization_id', organization.id),
    )
}

export const deleteOrganization = async (organization: Organization) => {
    await Organization.deleteById(organization.id)
}

export const requireOrganizationRole = (admin: Admin | JwtAdmin, minRole: OrganizationRole) => {
    if (organizationRoles.indexOf(admin.role) < organizationRoles.indexOf(minRole)) {
        throw new RequestError(`Minimum organization role ${minRole} is required`, 403)
    }
}

export const organizationRoleMiddleware = (minRole: OrganizationRole) => async (ctx: ParameterizedContext<{ admin?: Admin | JwtAdmin }>, next: Next) => {
    requireOrganizationRole(ctx.state.admin!, minRole)
    return next()
}
