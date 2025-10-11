import { AnyJson } from 'rules/Rule'
import Model from '../core/Model'
import { type JourneyStep } from './JourneyStep'
import { UUID } from 'node:crypto'

export default class JourneyUserStep extends Model {
    user_id!: UUID
    type!: string
    journey_id!: UUID
    step_id!: UUID
    delay_until?: Date | null
    entrance_id?: UUID
    ended_at?: Date
    data?: Record<string, AnyJson> | null
    ref?: string

    step?: JourneyStep

    static tableName = 'journey_user_step'

    static jsonAttributes = ['data']
    static virtualAttributes = ['step']

    static getDataMap(steps: JourneyStep[], userSteps: JourneyUserStep[]) {
        return userSteps.reduceRight<Record<string, AnyJson>>((a, { data, step_id }) => {
            const step = steps.find(s => s.id === step_id)
            if (data && step && !a[step.dataKey]) {
                a[step.dataKey] = data
            }
            return a
        }, {})
    }
}
