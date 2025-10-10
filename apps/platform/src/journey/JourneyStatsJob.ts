import { UUID } from 'node:crypto'
import { Job } from '../queue'
import Journey from './Journey'
import { JourneyStep } from './JourneyStep'
import JourneyUserStep from './JourneyUserStep'

interface JourneyStatsParams {
    journey_id: UUID
}

export default class JourneyStatsJob extends Job {
    static $name = 'journey_stats_job'

    static from(journey_id: UUID) {
        return new this({ journey_id })
    }

    static async handler({ journey_id }: JourneyStatsParams) {
        const stats_at = new Date()

        const [steps, counts] = await Promise.all([
            JourneyStep.query()
                .select('id')
                .where('journey_id', journey_id),
            JourneyUserStep.query()
                .select('step_id')
                .sum({
                    entrance: JourneyUserStep.raw('case when entrance_id is null then 1 else 0 end'),
                    ended: JourneyUserStep.raw('case when entrance_id is null and ended_at is not null then 1 else 0 end'),
                    completed: JourneyUserStep.raw('case when type = ? then 1 else 0 end', ['completed']),
                    error: JourneyUserStep.raw('case when type = ? then 1 else 0 end', ['error']),
                    delay: JourneyUserStep.raw('case when type = ? then 1 else 0 end', ['delay']),
                    action: JourneyUserStep.raw('case when type = ? then 1 else 0 end', ['action']),
                })
                .where('journey_id', journey_id)
                .groupBy('step_id') as Promise<Array<{
                    step_id: UUID
                    entrance: number
                    ended: number
                    completed: number
                    error: number
                    delay: number
                    action: number
                }>>,
        ])

        // knex returns the sums as strings for some reason
        counts.forEach(o => Object.entries(o).forEach(([stat, count]) => {
            if (stat !== 'step_id') {
                (o as any)[stat] = Number(count)
            }
        }))

        await Journey.update(q => q.where('id', journey_id), {
            stats: counts.reduce((a, { step_id, ...rest }) => {
                for (const [stat, count] of Object.entries(rest)) {
                    a[stat] = (a[stat] ?? 0) + count
                }
                return a
            }, {
                entrance: 0,
                ended: 0,
                completed: 0,
                error: 0,
                delay: 0,
                action: 0,
            } as Record<string, number>),
            stats_at,
        })

        for (const step of steps) {
            const { step_id, ...stats } = counts.find(c => c.step_id === step.id) ?? {}
            await JourneyStep.update(q => q.where('id', step.id), {
                stats,
                stats_at,
            })
        }
    }

}
