import { UUID } from 'crypto'
import { Job } from '../queue'
import List from './List'
import { getList, listUserCount } from './ListService'

interface ListStatsParams {
    listId: UUID
    projectId: UUID
}

export default class ListStatsJob extends Job {
    static $name = 'list_stats_job'

    static from(
        listId: UUID,
        projectId: UUID,
    ): ListStatsJob {
        return new this({ listId, projectId }).deduplicationKey(`${this.$name}_${listId}`)
    }

    static async handler({ listId, projectId }: ListStatsParams) {

        const list = await getList(listId, projectId)
        if (!list) return

        // Update the list with the new totals
        await List.query()
            .update({ users_count: await listUserCount(list) })
            .where('id', listId)
    }
}
