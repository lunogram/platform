import { UUID } from 'crypto'
import Model from '../Model'

export default class Audit extends Model {
    project_id!: UUID
    admin_id?: UUID
    event!: string
    object!: Record<string, any>
    object_changes!: Record<string, any>
    item_id!: UUID
    item_type!: string

    static jsonAttributes = ['object', 'object_changes']
}

export interface Auditable {
    id: UUID
    $tableName: string
}

export type WithAdmin<T> = T & {
    admin_id?: UUID
}
