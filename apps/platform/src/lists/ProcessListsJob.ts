import { Job } from '../queue'
import List from './List'
import ListStatsJob from './ListStatsJob'

export default class ProcessListsJob extends Job {
    static $name = 'process_lists_job'

    static from(): ProcessListsJob {
        return new this().deduplicationKey(this.$name)
    }

    static async handler() {

        const lists = await List.all(qb => qb.whereNot('state', 'loading').whereNull('deleted_at'))
        for (const list of lists) {

            // Update stats on all lists
            await ListStatsJob.from(list.id, list.project_id).queue()
        }
    }
}
