import { UUID } from 'crypto'
import Model, { ModelParams } from '../core/Model'

export class Tag extends Model {
    project_id!: UUID
    name!: string
}

export class EntityTag extends Model {
    entity!: string // table name
    entity_id!: UUID
    tag_id!: UUID
}

export type TagParams = Omit<Tag, ModelParams | 'project_id'>
