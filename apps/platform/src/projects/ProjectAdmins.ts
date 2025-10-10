import { UUID } from 'node:crypto'
import Model from '../core/Model'
import { ProjectRole } from './Project'

export class ProjectAdmin extends Model {
    project_id!: UUID
    admin_id?: UUID
    role!: ProjectRole
    deleted_at?: Date
}

export type ProjectAdminParams = Pick<ProjectAdmin, 'role'>
