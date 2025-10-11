import { UUID } from 'node:crypto'
import App from '../app'
import { acquireLock, releaseLock } from '../core/Lock'
import { getProject } from '../projects/ProjectService'
import Job from '../queue/Job'
import { Rule } from '../rules/Rule'
import { User } from '../users/User'
import { getUserEventsForRules } from '../users/UserRepository'
import { shallowEqual } from '../utilities'
import { getEntranceSubsequentSteps, getJourneyStepChildren, getJourneySteps } from './JourneyRepository'
import { JourneyStep, JourneyStepChild, journeyStepTypes } from './JourneyStep'
import JourneyUserStep from './JourneyUserStep'
import { logger } from '../config/logger'

type JobOrJobFunc = Job | ((state: JourneyState) => Promise<Job | undefined>)

export class JourneyState {

    /**
     * Resumes journey sequence/cycle processing for a given entrance (user can have multiple entrances, be in the journey multiple times)
     * @param entrance entrance user step
     * @param user target user to run journey for
     * @returns promise that resolves when processing ends
     */
    public static async resume(entrance: UUID | JourneyUserStep, user?: User) {
        if (typeof entrance === 'string') {
            entrance = (await JourneyUserStep.find(entrance))!
        }
        if (!entrance) {
            return
        }
        if (entrance.entrance_id) {
            entrance = (await JourneyUserStep.find(entrance.entrance_id))!
            if (!entrance || entrance.entrance_id) {
                return
            }
        }

        logger.debug('resuming journey', { entrance_id: entrance.id })

        if (entrance.ended_at) {
            logger.debug('attempt to resume ended journey', { entrance_id: entrance.id })
            return
        }

        // Find user
        if (!user) {
            user = await User.find(entrance.user_id)
        }
        if (!user) {
            return
        }

        // User-entrance mismatch
        if (entrance.user_id !== user.id) {
            return
        }

        const key = `journey:entrance:${entrance.id}`

        const acquired = await acquireLock({ key })
        if (!acquired) {
            return
        }

        // Load all journey dependencies
        const [steps, children, userSteps] = await Promise.all([
            getJourneySteps(entrance.journey_id)
                .then(steps => steps.map(s => journeyStepTypes[s.type]?.fromJson(s))),
            getJourneyStepChildren(entrance.journey_id),
            getEntranceSubsequentSteps(entrance.id),
        ])

        const state = new this(entrance, steps, children, [entrance, ...userSteps], user)

        await state.run()

        await releaseLock(key)

        return state
    }

    private _timezone?: string

    // Batch enqueue jobs after processing
    private _jobs: JobOrJobFunc[] = []

    constructor(
        public readonly entrance: JourneyUserStep,
        public readonly steps: JourneyStep[],
        public readonly children: JourneyStepChild[],
        public readonly userSteps: JourneyUserStep[],
        public readonly user: User,
    ) { }

    private async run() {
        let currentStep = this.userSteps[this.userSteps.length - 1]
        let step = this.steps.find(s => s.id === currentStep.step_id)

        while (step) {
            // NOTE: we have to check if we advanced to a new step. A pending JourneyUserStep
            // is created the type of the step will be updated once processed.
            if (currentStep.step_id !== step.id) {
                this.userSteps.push(currentStep = JourneyUserStep.fromJson({
                    journey_id: this.entrance.journey_id,
                    entrance_id: this.entrance.id,
                    user_id: this.user.id,
                    step_id: step.id,
                }))
            }

            // continue on if this step is completed
            if (currentStep.type === 'completed') {
                step = await this.next(step)
                continue
            }

            const copy = { ...currentStep }

            // delegate to step type
            try {
                await step.process(this, currentStep)
            } catch (err) {
                currentStep.type = 'error'
            }

            // persist and update the user step
            if (currentStep.id) {
                // only update the step is something has changed
                if (!shallowEqual(copy, currentStep)) {
                    currentStep.parseJson(await JourneyUserStep.updateAndFetch(currentStep.id, currentStep))
                }
            } else {
                currentStep.parseJson(await JourneyUserStep.insertAndFetch(currentStep))
            }

            // Stop processing if latest isn't completed
            if (currentStep.type !== 'completed') {
                // Exit journey completely if a catastrophic error
                // has occurred to avoid unpredictable behavior
                if (currentStep.type === 'error') {
                    await this.end()
                }
                break
            }
        }

        if (this._jobs.length) {
            const jobs: Job[] = []
            for (let j of this._jobs) {
                if (typeof j === 'function') {
                    const i = await j(this)
                    if (!i) continue
                    j = i
                }
                jobs.push(j)
            }
            await App.main.queue.enqueueBatch(jobs)
        }
    }

    private async next(step: JourneyStep) {
        const nextId = await step.next(this)
        if (!nextId) {
            await this.end()
            return
        }

        const next = this.steps.find(s => s.id === nextId)
        if (!next) {
            await this.end()
            return
        }

        // NOTE: we want to end circular reference within this entrance
        const revisited = this.userSteps.find(
            s => s.step_id === next.id,
        )

        // TODO: the problem with returning a revisited
        if (revisited && revisited.type === 'completed') {
            await this.end()
            return
        }

        return next
    }

    private async end() {
        await JourneyUserStep.update(q => q.where('id', this.entrance.id), {
            ended_at: new Date(),
        })
    }

    public childrenOf(stepId: UUID) {
        return this.children.filter(sc => sc.step_id === stepId)
    }

    public job(job: JobOrJobFunc) {
        this._jobs.push(job)
    }

    public async events(rule: Rule) {
        // TODO: Find a way to not have to pull in all events, better discern
        return await getUserEventsForRules(this.user.id, rule)
    }

    public async timezone() {
        if (!this._timezone) {
            this._timezone = this.user.timezone
        }
        if (!this._timezone) {
            this._timezone = (await getProject(this.user.project_id))!.timezone
        }
        return this._timezone!
    }

    public stepData() {
        return JourneyUserStep.getDataMap(this.steps, this.userSteps)
    }
}
