import App from '../app'
import { Job } from '../queue'
import { migrateEvents, migrateLists, migrateUsers } from '../utilities/migrate'

interface MigrateJobParams {
    type: 'lists' | 'events' | 'users'
    since?: string | Date | undefined
    user_id?: number
}

export default class MigrateJob extends Job {

    static $name = 'migrate_job'

    static from(params: MigrateJobParams) {
        return new MigrateJob(params)
    }

    static async handler({ type, since, user_id }: MigrateJobParams) {
        if (type === 'lists') {
            await App.main.redis.set('migration:lists', JSON.stringify(true))
            await migrateLists()
        } else if (type === 'events') {
            await App.main.redis.set('migration:events', JSON.stringify(true))
            await migrateEvents(since ? new Date(since) : undefined)
        } else if (type === 'users') {
            await App.main.redis.set('migration:users', JSON.stringify(true))
            await migrateUsers(since ? new Date(since) : undefined, user_id)
        }
    }
}
