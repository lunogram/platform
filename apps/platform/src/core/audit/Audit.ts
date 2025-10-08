import Model from '../Model'

export default class Audit extends Model {
    project_id!: number
    admin_id?: number
    event!: string
    object!: Record<string, any>
    object_changes!: Record<string, any>
    item_id!: number
    item_type!: string

    static jsonAttributes = ['object', 'object_changes']
}

export interface Auditable {
    id: number
    $tableName: string
}

export type WithAdmin<T> = T & {
    admin_id?: number
}
