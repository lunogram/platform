import { pluralize, snakeCase } from '../../utilities'

export class RawModel {
    get $tableName() {
        return (this.constructor as typeof RawModel).tableName
    }

    static jsonAttributes: string[] = []
    static virtualAttributes: string[] = []

    static toJson<T extends typeof RawModel>(this: T, model: any) {
        const json: any = {}
        const keys = [...Object.keys(model), ...this.virtualAttributes]
        for (const key of keys) {
            json[snakeCase(key)] = model[key]
        }
        return json
    }

    static fromJson<T extends typeof RawModel>(this: T, json: Partial<InstanceType<T>>): InstanceType<T> {
        const model = new this()

        // Remove any value that could conflict with a virtual key
        for (const attribute of this.virtualAttributes) {
            delete (json as any)[attribute]
        }

        // Parse values into the model
        model.parseJson(json)
        return model as InstanceType<T>
    }

    parseJson(json: any) {
        Object.assign(this, json)
    }

    toJSON() {
        return (this.constructor as any).toJson(this)
    }

    // Format JSON before inserting into DB
    static formatJson(json: any): Record<string, unknown> {

        // remove any virtual attributes that have been set
        for (const attribute of this.virtualAttributes) {
            delete (json as any)[attribute]
        }

        return json
    }

    static get tableName(): string {
        return pluralize(snakeCase(this.name))
    }
}
