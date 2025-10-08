import { EncodedJob, Job } from '../queue'
import { saveDevice } from './UserRepository'
import { DeviceParams } from './Device'
import { LockError } from '../core/Lock'
import App from '../app'

type UserDeviceTrigger = DeviceParams & {
    project_id: number
}

export default class UserDeviceJob extends Job {
    static $name = 'user_register_device'

    static from(data: UserDeviceTrigger): UserDeviceJob {
        return new this(data)
    }

    static async handler({ project_id, ...device }: UserDeviceTrigger, raw: EncodedJob) {
        const attempts = raw.options.attempts ?? 1
        const attemptsMade = raw.state.attemptsMade ?? 0

        try {
            await saveDevice(project_id, device)
        } catch (error) {

            // If record is locked, re-queue the job
            if (error instanceof LockError) {
                await App.main.queue.retry(raw)
                throw error
            }

            if (attemptsMade < (attempts - 1)) throw error
        }
    }
}
