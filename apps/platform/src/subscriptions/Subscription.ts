import { ChannelType } from '../config/channels'
import Model, { ModelParams } from '../core/Model'

export default class Subscription extends Model {
    project_id!: number
    name!: string
    channel!: ChannelType
    is_public!: boolean
}

export enum SubscriptionState {
    unsubscribed = 0,
    subscribed = 1,
    optedIn = 2,
}

export type UserSubscription = {
    subscription_id: number
    state: SubscriptionState
    name: string
    channel: string
}

export type SubscriptionParams = Omit<Subscription, ModelParams>
export type SubscriptionUpdateParams = Pick<SubscriptionParams, 'name' | 'is_public'>
