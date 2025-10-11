import { Database } from 'config/database'
import App from '../app'
import { SQLModel } from './models/SQLModel'
import { UUID } from 'crypto'

export interface SearchResult<T> {
    results: T[]
    nextCursor?: string
    prevCursor?: string
    limit: number
}

export * from './models/RawModel'
export * from './models/SQLModel'

export default class Model extends SQLModel {
    id!: UUID

    static async findMap<T extends typeof Model>(
        this: T,
        ids: UUID[],
        db: Database = App.main.db,
    ) {
        const m = new Map<UUID, InstanceType<T>>()
        if (!ids.length) return m
        const records = await this.all(q => q.whereIn('id', ids), db)
        for (const record of records) {
            m.set(record.id, record)
        }
        return m
    }
}

export type ModelParams = 'id' | 'created_at' | 'updated_at' | 'parseJson' | 'project_id' | 'toJSON' | '$tableName'
