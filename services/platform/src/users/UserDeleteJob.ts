import { UUID } from 'crypto'
import { Job } from '../queue'
import { deleteUserByExternalId, deleteUserById } from './UserRepository'

interface UserDeleteTrigger {
    project_id: UUID
    id?: UUID
    external_id?: string
}

export default class UserDeleteJob extends Job {
    static $name = 'user_delete'

    static from(data: UserDeleteTrigger): UserDeleteJob {
        return new this(data)
    }

    static async handler({ project_id, id, external_id }: UserDeleteTrigger) {
        if (external_id) await deleteUserByExternalId(project_id, external_id)
        if (id) await deleteUserById(project_id, id)
    }
}
