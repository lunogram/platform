import { UUID } from 'crypto'
import Model, { ModelParams } from '../core/Model'

export default class Project extends Model {
    organization_id!: UUID
    name!: string
    description?: string
    deleted_at?: Date
    locale!: string
    timezone!: string
    text_opt_out_message?: string
    text_help_message?: string
    link_wrap_email?: boolean
    link_wrap_push?: boolean
    campaigns_count?: number
    journeys_count?: number
    users_count?: number
}

export type ProjectParams = Omit<Project, ModelParams | 'deleted_at' | 'organization_id' | 'campaigns_count' | 'journeys_count' | 'users_count'>

export const projectRoles = [
    'support',
    'editor',
    'publisher',
    'admin',
] as const

export type ProjectRole = (typeof projectRoles)[number]
