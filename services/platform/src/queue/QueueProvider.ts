import Queue from './Queue'
import { EncodedJob } from './Job'

export type QueueProviderName = 'redis' | 'memory' | 'logger'

export interface Metric {
    date: Date
    count: number
}

export default interface QueueProvider {
    queue: Queue
    batchSize: number
    enqueue(job: EncodedJob): Promise<void>
    enqueueBatch(jobs: EncodedJob[]): Promise<void>
    delay(job: EncodedJob, milliseconds: number): Promise<void>
    retry(job: EncodedJob): Promise<void>
    start(): void
    pause(): Promise<void>
    resume(): Promise<void>
    isRunning(): Promise<boolean>
    close(): void
    failed?(): Promise<any>
}
