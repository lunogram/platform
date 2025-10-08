import { User } from '../users/User'
import { getRuleQuery, make } from '../rules/RuleEngine'
import List, { ListCreateParams, ListUpdateParams, ListVersion } from './List'
import { PageParams } from '../core/searchParams'
import { importUsers } from '../users/UserImport'
import { FileStream } from '../storage/FileStream'
import { createTagSubquery, getTags, setTags } from '../tags/TagService'
import { pick } from '../utilities'
import { fetchAndCompileRule } from '../rules/RuleService'
import { createEvent } from '../users/UserEventRepository'

export const pagedLists = async (params: PageParams, projectId: number) => {
    const result = await List.search(
        { ...params, fields: ['name'] },
        b => {
            b = b.where('project_id', projectId)
                .whereNull('deleted_at')
                .where('is_visible', true)
            params.tag?.length && b.whereIn('id', createTagSubquery(List, projectId, params.tag))
            return b
        },
    )
    if (result.results?.length) {
        const tags = await getTags(List.tableName, result.results.map(l => l.id))
        for (const list of result.results) {
            list.tags = tags.get(list.id)
        }
    }
    return result
}

export const allLists = async (projectId: number, listIds?: number[]) => {
    const lists = await List.all(qb => {
        qb.where('project_id', projectId)
        if (listIds) {
            qb.whereIn('id', listIds)
        }
        return qb
    })

    if (lists.length) {
        const tags = await getTags(List.tableName, lists.map(l => l.id))
        for (const list of lists) {
            list.tags = tags.get(list.id)
        }
    }

    return lists
}

export const getList = async (id: number, projectId: number) => {
    const list = await List.find(id, qb => qb.where('project_id', projectId))
    if (list) {
        list.tags = await getTags(List.tableName, [list.id]).then(m => m.get(list.id))
        if (list.rule_id && !list.rule) list.rule = await fetchAndCompileRule(list.rule_id)
    }

    return list
}

export const getListUsers = async (list: List, params: PageParams, projectId: number) => {

    const limit = params.limit ?? 25
    const offset = parseInt(params.cursor ?? '0') ?? 0
    const subquery = getRuleQuery(projectId, list.rule)
    const query = `
        SELECT DISTINCT id FROM (${subquery})
        ORDER BY id DESC
        LIMIT {limit: UInt32}
        OFFSET {offset: UInt32}
    `
    const results = await User.clickhouse().all(
        query,
        {
            projectId,
            limit,
            offset,
        },
    )
    const users = await User.all(query => query.whereIn('id', results.map(r => r.id)))
    return {
        results: users,
        limit,
        prevCursor: offset > 0 ? `${Math.max(0, offset - limit)}` : undefined,
        nextCursor: results.length < limit ? undefined : `${offset + limit}`,
    }
}

export const createList = async (projectId: number, { tags, name, type, rule }: ListCreateParams): Promise<List> => {
    let list = await List.insertAndFetch({
        name,
        type,
        state: type === 'dynamic' ? 'draft' : 'ready',
        users_count: 0,
        project_id: projectId,
        rule,
    })

    if (type === 'static') {
        list = await List.updateAndFetch(list.id, {
            rule: staticListRule(list),
        })
    }

    if (tags?.length) {
        await setTags({
            project_id: projectId,
            entity: List.tableName,
            entity_id: list.id,
            names: tags,
        })
    }

    return list
}

export const updateList = async (list: List, { tags, published, ...params }: ListUpdateParams): Promise<List | undefined> => {
    list = await List.updateAndFetch(list.id, {
        ...params,
        state: list.state === 'draft'
            ? published
                ? 'ready'
                : 'draft'
            : list.state,
        users_count: await listUserCount({
            project_id: list.project_id,
            rule: params.rule ?? list.rule,
        }),
    })

    if (tags) {
        await setTags({
            project_id: list.project_id,
            entity: List.tableName,
            entity_id: list.id,
            names: tags,
        })
    }

    return await getList(list.id, list.project_id)
}

export const archiveList = async (id: number, projectId: number) => {
    await List.archive(id, qb => qb.where('project_id', projectId))
    return getList(id, projectId)
}

export const deleteList = async (id: number, projectId: number) => {
    return await List.deleteById(id, qb => qb.where('project_id', projectId))
}

export const staticListRule = (list: List) => {
    return make({
        type: 'wrapper',
        operator: 'and',
        group: 'parent',
        children: [make({
            path: '$.name',
            value: 'user_imported_to_list',
            type: 'wrapper',
            group: 'event',
            operator: 'and',
            children: [
                make({
                    path: '$.list_id',
                    type: 'number',
                    group: 'event',
                    value: list.id,
                }),
                make({
                    path: '$.version',
                    type: 'number',
                    group: 'event',
                    value: list.version,
                }),
            ],
        })],
    })
}

export const importUsersToList = async (list: List, stream: FileStream) => {
    await updateListState(list.id, { state: 'loading' })

    try {
        await importUsers({
            project_id: list.project_id,
            list_id: list!.id,
            stream,
        })
    } finally {
        await updateListState(list.id, { state: 'ready' })
    }

    await updateListState(list.id, { state: 'ready' })
}

export const addUserToList = async (user: User, list: ListVersion) => {
    await createEvent(user, {
        name: 'user_imported_to_list',
        data: {
            list_id: list.id,
            version: list.version,
        },
    }, false)
}

export const listUserCount = async (list: Pick<List, 'project_id' | 'rule'>): Promise<number> => {
    return await User.clickhouse().count(
        getRuleQuery(list.project_id, list.rule),
    )
}

export const updateListState = async (id: number, params: Partial<Pick<List, 'state' | 'version' | 'users_count' | 'refreshed_at'>>) => {
    return await List.updateAndFetch(id, params)
}

export const duplicateList = async (list: List) => {
    const params: Partial<List> = pick(list, ['project_id', 'name', 'type', 'rule_id', 'rule', 'is_visible'])
    params.name = `Copy of ${params.name}`
    params.state = 'draft'
    return await List.insertAndFetch(params)
}
