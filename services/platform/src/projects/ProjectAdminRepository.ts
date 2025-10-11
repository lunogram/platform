import { Database } from 'config/database'
import { PageParams } from '../core/searchParams'
import { ProjectRole } from './Project'
import { ProjectAdmin } from './ProjectAdmins'
import { UUID } from 'node:crypto'

const adminSelectFields = ['admins.first_name', 'admins.last_name', 'admins.email']
const projectAdminFields = [`${ProjectAdmin.tableName}.*`, ...adminSelectFields]

const baseProjectAdminQuery = (builder: Database.QueryBuilder<any>, projectId: UUID) => {
    return builder
        .select(projectAdminFields)
        .join('admins', 'admin_id', '=', 'admins.id')
        .where('project_id', projectId)
        .whereNull(`${ProjectAdmin.tableName}.deleted_at`)
}

export const pagedProjectAdmins = async (params: PageParams, projectId: UUID) => {
    return await ProjectAdmin.search(
        { ...params, fields: adminSelectFields },
        q => baseProjectAdminQuery(q, projectId),
    )
}

export const getProjectAdmin = async (projectId: UUID, adminId: UUID) => {
    return await ProjectAdmin.first(q => baseProjectAdminQuery(q.where('admin_id', adminId), projectId))
}

export const addAdminToProject = async (projectId: UUID, adminId: UUID, role: ProjectRole) => {
    const admin = await getProjectAdmin(projectId, adminId)
    if (admin) {
        return await ProjectAdmin.update(
            qb => qb.where('project_id', projectId)
                .where('admin_id', adminId),
            { role },
        )
    }
    return await ProjectAdmin.insert({
        admin_id: adminId,
        project_id: projectId,
        role,
    })
}

export const removeAdminFromProject = async (projectId: UUID, adminId: UUID) => {
    return await ProjectAdmin.update(
        qb => qb.where('admin_id', adminId).where('project_id', projectId),
        { deleted_at: new Date() },
    )
}
