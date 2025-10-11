import { Job } from '../queue'
import App from '../app'
import { chunk } from '../utilities'
import JourneyUserStep from './JourneyUserStep'
import Journey from './Journey'
import JourneyProcessJob from './JourneyProcessJob'
import { UUID } from 'node:crypto'

interface JourneyDelayJobParams {
    journey_id: UUID
}

export default class JourneyDelayJob extends Job {
    static $name = 'journey_delay_job'

    static async enqueueActive(app: App) {
        const query = Journey.query(app.db)
            .select('id')
            .whereNot('status', 'off')
            .whereNull('deleted_at')
        await chunk<{ id: UUID }>(query, app.queue.batchSize, async journeys => {
            app.queue.enqueueBatch(journeys.map(({ id }) => JourneyDelayJob.from(id)))
        })
    }

    static from(journey_id: UUID) {
        return new JourneyDelayJob({ journey_id })
    }

    static async handler({ journey_id }: JourneyDelayJobParams) {
        if (!journey_id) return

        const { queue } = App.main

        const updated = await JourneyUserStep
            .query()
            .where('journey_id', journey_id)
            .where('type', 'delay')
            .where('delay_until', '<=', 'NOW()')
            .returning(['id'])

        if (updated.length === 0) return

        await queue.enqueueBatch(updated.map(({ id }) => JourneyProcessJob.from({ entrance_id: id })))
    }
}
