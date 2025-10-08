import { RequireAtLeastOne } from '../Types'
import { deepDiff } from '../../utilities'
import Audit, { Auditable } from './Audit'

type AuditParams = {
    project_id: number
    event: string
    item_id: number
    item_type: string
}

type AuditCreateParams = RequireAtLeastOne<{
    project_id: number
    event: string
    admin_id?: number
} & ({
    object?: Auditable
    previous?: Record<string, any>
} | {
    object?: Record<string, any>
    previous?: Auditable
}), 'object' | 'previous'>

export const createAuditLog = async (data: AuditCreateParams) => {
    return await Audit.insert({
        project_id: data.project_id,
        event: data.event,
        admin_id: data.admin_id,
        object: data.object,
        object_changes: deepDiff(data.object ?? {}, data.previous ?? {}),
        item_id: data.object?.id ?? data.previous?.id,
        item_type: data.object?.$tableName ?? data.previous?.$tableName,
    })
}

export const getAuditLogs = async ({
    project_id,
    event,
    item_id,
    item_type,
}: AuditParams) => {
    return Audit.all(q =>
        q.where('project_id', project_id)
            .where('event', event)
            .where('item_id', item_id)
            .where('item_type', item_type)
            .orderBy('created_at', 'desc'),
    )
}

export const getLastAuditLog = async ({
    project_id,
    event,
    item_id,
    item_type,
}: AuditParams) => {
    return Audit.first(q =>
        q.where('project_id', project_id)
            .where('event', event)
            .where('item_id', item_id)
            .where('item_type', item_type)
            .orderBy('created_at', 'desc'),
    )
}
