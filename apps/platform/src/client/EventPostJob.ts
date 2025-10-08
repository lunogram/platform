import { getUser, getUserFromClientId } from '../users/UserRepository'
import { ClientIdentity, ClientPostEvent } from './Client'
import { Job } from '../queue'
import { createEvent } from '../users/UserEventRepository'
import { enterJourneysFromEvent } from '../journey/JourneyService'
import { UserPatchJob } from '../jobs'
import { User } from '../users/User'

interface EventPostTrigger {
    project_id: number
    user_id?: number
    event: ClientPostEvent
    forward?: boolean
}

export default class EventPostJob extends Job {
    static $name = 'event_post'

    options = {
        delay: 0,
        attempts: 2,
    }

    static from(data: EventPostTrigger): EventPostJob {
        return new this(data)
    }

    static async handler({ project_id, user_id, event: clientEvent, forward = false }: EventPostTrigger) {
        const { anonymous_id, external_id } = clientEvent
        const identity = { external_id, anonymous_id } as ClientIdentity
        let user = user_id
            ? await getUser(user_id, project_id)
            : await getUserFromClientId(project_id, identity)

        // If no user exists, create one if we have enough information
        if (!user || clientEvent.user) {
            user = await UserPatchJob.from({
                project_id,
                user: { ...(clientEvent.user ?? {}), ...identity },
            }).handle<User>()
        }

        // Create event for given user
        const event = await createEvent(user, {
            name: clientEvent.name,
            data: clientEvent.data || {},
        }, forward)

        // Enter any journey entrances associated with this event
        await enterJourneysFromEvent(event, user)

        return { user, event }
    }
}
