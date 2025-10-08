import { Job } from '../queue'
import { deleteUser } from './UserRepository'

interface UserDeleteTrigger {
    project_id: number
    external_id: string
}

export default class UserDeleteJob extends Job {
    static $name = 'user_delete'

    static from(data: UserDeleteTrigger): UserDeleteJob {
        return new this(data)
    }

    static async handler({ project_id, external_id }: UserDeleteTrigger) {
        await deleteUser(project_id, external_id)
    }
}
