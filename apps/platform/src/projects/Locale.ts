import { UUID } from 'crypto'
import Model, { ModelParams } from '../core/Model'

export default class Locale extends Model {
    project_id!: UUID
    key!: string
    label!: string
}

export type LocaleParams = Omit<Locale, ModelParams | 'project_id'>
