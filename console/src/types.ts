import type { ComponentType, Dispatch, Key, ReactNode, SetStateAction } from "react"
import type { FieldPath, FieldValues, UseFormReturn } from "react-hook-form"
import type { Node } from "reactflow"
import type { UUID } from "@/types/common"

export type Class<T> = new () => T

export interface ControlledProps<T> {
    value: T
    onChange: (value: T) => void
}

export interface CommonInputProps {
    required?: boolean
    disabled?: boolean
    label?: ReactNode
    subtitle?: ReactNode
    hideLabel?: boolean
    error?: ReactNode
}

export type ControlledInputProps<T> = ControlledProps<T> & CommonInputProps

export interface FieldProps<
    X extends FieldValues,
    P extends FieldPath<X>,
> extends CommonInputProps {
    form: UseFormReturn<X>
    name: P
}

export type FieldBindingsProps<
    I extends ControlledInputProps<T>,
    T,
    X extends FieldValues,
    P extends FieldPath<X>,
> = Omit<I, keyof ControlledProps<T>> & FieldProps<X, P>

export interface OptionsProps<O, V = O> {
    options: O[] | readonly O[]
    toValue?: (option: O) => V
    getValueKey?: (option: V) => Key
    getOptionDisplay?: (option: O) => ReactNode
}

export type UseStateContext<T> = [T, Dispatch<SetStateAction<T>>]

export interface OAuthResponse {
    refresh_token: string
    access_token: string
    expires_at: Date
    refresh_expires_at: Date
}

export type Operator =
    | "="
    | "!="
    | "<"
    | "<="
    | ">"
    | ">="
    | "="
    | "is set"
    | "is not set"
    | "or"
    | "and"
    | "xor"
    | "empty"
    | "contains"
    | "not contain"
    | "starts with"
    | "not start with"
    | "any"
    | "none"
    | "is same day"
export type RuleType = "wrapper" | "string" | "number" | "boolean" | "date" | "array"
export type RuleGroup =
    | "user"
    | "event"
    | "parent"
    | "organization"
    | "organization_user"
    | "organization_event"

export type AnyJson = boolean | number | string | null | JsonArray | JsonMap
export interface JsonMap {
    [key: string]: AnyJson
}
export type JsonArray = AnyJson[]

export type Rule = {
    uuid: UUID
    root_uuid?: UUID
    parent_uuid?: UUID
    type: RuleType
    group: RuleGroup
    path: string
    operator: Operator
    value?: string
} & (
    | { type: "wrapper" }
    | { type: "string" }
    | { type: "number" }
    | { type: "boolean" }
    | { type: "date" }
    | { type: "array" }
)

export function defaultOperator(type: RuleType): Operator {
    switch (type) {
        case "string":
            return "="
        case "number":
            return "="
        case "boolean":
            return "="
        case "date":
            return "="
        case "array":
            return "any"
        case "wrapper":
            return "and"
    }
}

export function typeOperators(type: RuleType): Operator[] {
    switch (type) {
        case "string":
            return [
                "=",
                "!=",
                "contains",
                "not contain",
                "starts with",
                "not start with",
                "is set",
                "is not set",
                "empty",
            ]
        case "number":
            return ["=", "!=", "<", "<=", ">", ">=", "is set", "is not set", "empty"]
        case "boolean":
            return ["=", "!=", "is set", "is not set", "empty"]
        case "date":
            return ["=", "!=", "<", "<=", ">", ">=", "is same day", "is set", "is not set", "empty"]
        case "array":
            return ["any", "none", "is set", "is not set", "empty"]
        case "wrapper":
            return ["and", "or", "xor"]
    }
}

export type WrapperRule = Rule & { type: "wrapper"; children: Rule[] }

export type EventRulePeriod =
    | {
          type: "rolling"
          unit: "minute" | "hour" | "day" | "week" | "month"
          value: number
      }
    | {
          type: "fixed"
          start_date: string
          end_date?: string
      }

export interface EventRuleFrequency {
    period: EventRulePeriod
    operator: Operator
    count: number | undefined
}

export type EventRule = {
    group: "event"
    frequency?: EventRuleFrequency
} & WrapperRule

// Organization Event Rule - matches users who belong to organizations
// that have triggered specific events
export type OrganizationUserMatchType =
    | "all" // All members of the organization
    | "conditions" // Members matching property conditions on membership data

export interface OrganizationUserMatch {
    type: OrganizationUserMatchType
    // For condition-based matching - rules applied to organization membership data
    member_conditions?: WrapperRule
}

export type OrganizationEventRule = {
    group: "organization_event"
    frequency?: EventRuleFrequency
    // How to match users within the organization
    user_match: OrganizationUserMatch
} & WrapperRule

// Organization Rule - matches users who belong to organizations
// that have specific properties
export type OrganizationRule = {
    group: "organization"
    // How to match users within the organization
    user_match?: OrganizationUserMatch
} & WrapperRule

export interface RulePath {
    id: UUID
    path: string
    type: "user" | "event"
    name: string
    data_type: "string" | "number" | "boolean" | "date" | "array"
    visibility: "public" | "hidden" | "classified"
}

export interface UserSchemaPath {
    path: string
    types: string[]
}

export interface EventSchemaPath {
    path: string
    types: string[]
}

export interface EventSchema {
    id: UUID
    name: string
    schema: EventSchemaPath[]
}

export interface OrganizationUserSchemaPath {
    path: string
    types: string[]
}

export interface OrganizationSchemaPath {
    path: string
    types: string[]
}

export interface VariableSuggestions {
    userPaths: UserSchemaPath[]
    eventPaths: EventSchema[]
    organizationEventPaths?: EventSchema[]
    organizationUserPaths?: OrganizationUserSchemaPath[]
    organizationPaths?: OrganizationSchemaPath[]
}

export interface Preferences {
    readonly lang: string
    readonly mode: "light" | "dark"
    readonly timeZone: string
}

export interface SearchParams {
    cursor?: string
    page?: "next" | "prev"
    limit: number
    offset?: number
    sort?: string
    direction?: string
    filter?: Record<string, unknown>
    search?: string
    tag?: string[]
    id?: UUID[]
}

export interface SearchResult<T> {
    results: T[]
    nextCursor: string
    prevCursor?: string
    limit: number
    total?: number
    offset?: number
}

export type AuditFields = "created_at" | "updated_at" | "deleted_at"

export type AuthDriver = "basic" | "clerk"

export const AUTH_DRIVERS = {
    BASIC: "basic" as const,
    CLERK: "clerk" as const,
}

export const organizationRoles = ["member", "admin", "owner"] as const

export type OrganizationRole = (typeof organizationRoles)[number]

export interface Admin {
    id: UUID
    organization_id: UUID
    first_name: string
    last_name: string
    email: string
    image_url: string
    role: OrganizationRole
}

export interface Tenant {
    id: UUID
    username: string
    domain?: string
    auth: unknown
    tracking_deeplink_mirror_url?: string
}

export type TenantUpdateParams = Omit<Tenant, "id" | "auth" | AuditFields>

export const projectRoles = ["support", "editor", "publisher", "admin"] as const

export type ProjectRole = (typeof projectRoles)[number]

export interface ProjectAdmin extends Omit<Admin, "id" | "role"> {
    id: UUID
    created_at: string
    updated_at: string
    project_id: UUID
    admin_id: UUID
    role: ProjectRole
}

export type ProjectAdminParams = Pick<ProjectAdmin, "role">
export type ProjectAdminInviteParams = ProjectAdminParams & {
    email: string
}

export interface Project {
    id: UUID
    name: string
    description?: string
    locale: string
    timezone: string
    text_opt_out_message?: string
    text_help_message?: string
    link_wrap_email: boolean
    link_wrap_push: boolean
    created_at: string
    updated_at: string
    deleted_at?: string
    role?: ProjectRole
    has_provider?: boolean
    campaigns_count?: number
    journeys_count?: number
    users_count?: number
    lists_count?: number
}

export type ChannelType = "email" | "push" | "text"

export type ProjectCreate = Omit<Project, "id" | AuditFields>

export interface ProjectApiKey {
    id: UUID
    value: string
    name: string
    scope: "public" | "secret"
    role?: ProjectRole
    description?: string
}

export type ProjectApiKeyParams = Pick<ProjectApiKey, "name" | "description" | "scope" | "role">

export interface User {
    id: UUID
    anonymous_id?: string
    external_id?: string
    full_name?: string
    email?: string
    phone?: string
    timezone?: string
    locale?: string
    data: Record<string, unknown>
    devices?: Device[]
    created_at?: Date
}

export interface SubjectOrganization {
    id: UUID
    project_id: UUID
    external_id: string
    name?: string
    data: Record<string, unknown>
    version: number
    created_at: string
    updated_at: string
}

export type SubjectOrganizationCreateParams = Pick<
    SubjectOrganization,
    "external_id" | "name" | "data"
>
export type SubjectOrganizationUpdateParams = Pick<SubjectOrganization, "name" | "data">

export interface SubjectOrganizationMember {
    user_id: UUID
    organization_id: UUID
    data: Record<string, unknown>
    created_at: string
    updated_at: string
    user?: User
}

export type SubjectOrganizationMemberParams = Pick<SubjectOrganizationMember, "data"> & {
    user_id: UUID
}

export interface Device {
    device_id: string
    token?: string
    os: string
    model: string
    app_build: string
    app_version: string
}

export interface UserEvent {
    id: UUID
    name: string
    data: Record<string, unknown>
    created_at: string
}

export type ListState = "draft" | "ready" | "loading"
type ListType = "static" | "dynamic"

export type List = {
    id: UUID
    projectId: UUID
    name: string
    state: ListState
    type: ListType
    rule?: WrapperRule
    users_count: number
    tags?: string[]
    progress?: {
        complete: number
        total: number
    }
    is_visible: boolean
    created_at: string
    updated_at: string
} & (
    | {
          type: "dynamic"
          rule: WrapperRule
      }
    | { type: "static" }
)

export type DynamicList = List & { type: "dynamic" }

export type ListCreateParams = Pick<List, "name" | "rule" | "type" | "tags" | "is_visible">
export type ListUpdateParams = Pick<List, "name" | "rule" | "tags"> & {
    published?: boolean
}

type JourneyStatus = "draft" | "published" | "archived"

export interface Journey {
    id: UUID
    parent_id?: UUID
    draft_id?: UUID
    name: string
    description?: string
    template_id?: string
    status: JourneyStatus
    version_number?: number
    draft_version_id?: UUID
    published_version_id?: UUID
    tags?: string[]
    created_at: string
    updated_at: string
    deleted_at?: string
    stats_at?: string
    stats: Record<string, number>
}

export interface JourneyStep<T = any> {
    id: UUID
    type: string
    name: string
    data: T
    x: number
    y: number
}

export type JourneyStepParams = Omit<JourneyStep, "id">

interface JourneyStepMapChild<E = any> {
    external_id: string
    path?: string
    data?: E
}

export interface JourneyStepMap {
    [external_id: string]: {
        type: string
        name: string
        data_key?: string
        data?: Record<string, unknown>
        x: number
        y: number
        children?: JourneyStepMapChild[]
        stats?: Record<string, number>
        stats_at?: Date
        id?: UUID
    }
}

export interface JourneyStepTypeEditProps<T> extends ControlledProps<T> {
    journey: Journey
    project: Project
    stepId?: UUID // if already saved
}

export interface JourneyStepTypeEdgeProps<T, E> extends ControlledProps<E> {
    stepData: T
    siblingData: E[] // does not include self
    journey: Journey
    project: Project
}

export interface JourneyStepType<T = any, E = any> {
    name: string
    icon: ReactNode
    category: "entrance" | "delay" | "flow" | "action" | "exit" | "info"
    description: string
    Describe?: ComponentType<JourneyStepTypeEditProps<T>>
    newData?: () => Promise<T>
    newEdgeData?: () => Promise<E>
    Edit?: ComponentType<JourneyStepTypeEditProps<T> & { nodes: Node[] }>
    EditEdge?: ComponentType<JourneyStepTypeEdgeProps<T, E>>
    sources?: string[]
    multiChildSources?: boolean
    hasDataKey?: boolean
    hideTopHandle?: boolean
    hideBottomHandle?: boolean
    validate?: (data: T) => boolean
}

export interface JourneyUserStep {
    id: UUID
    entrance_id: UUID
    type: string
    delay_until?: string
    created_at: string
    updated_at: string
    ended_at?: string

    user?: User
    journey?: Journey
    step?: JourneyStep
}

export interface JourneyEntranceDetail {
    journey: Journey
    user: User
    userSteps: JourneyUserStep[]
}

export type CampaignState =
    | "draft"
    | "loading"
    | "scheduled"
    | "running"
    | "finished"
    | "aborting"
    | "aborted"

export interface CampaignDelivery {
    sent: number
    total: number
    opens: number
    clicks: number
}

export type CampaignType = "blast" | "trigger"

export interface Campaign {
    id: UUID
    project_id: UUID
    type: CampaignType
    name: string
    channel: ChannelType
    state: CampaignState
    delivery: CampaignDelivery
    provider_id?: UUID
    provider?: Provider
    subscription_id?: UUID
    subscription?: Subscription
    templates: Template[]
    list_ids?: UUID[]
    lists?: List[]
    exclusion_list_ids?: UUID[]
    exclusion_lists?: List[]
    tags?: string[]
    journeys?: Journey[]
    send_in_user_timezone: boolean
    send_at: string
    screenshot_url: string
    progress?: {
        complete: number
        total: number
    }
    created_at: string
    updated_at: string
}

export type CampaignSendState = "pending" | "sent" | "throttled" | "failed" | "bounced" | "aborted"

export type CampaignUpdateParams = Partial<
    Pick<
        Campaign,
        | "name"
        | "provider_id"
        | "state"
        | "list_ids"
        | "exclusion_list_ids"
        | "subscription_id"
        | "tags"
    >
>
export type CampaignCreateParams = Pick<Campaign, "name" | "channel" | "tags">
export type CampaignLaunchType = "now" | "later"
export type CampaignLaunchParams = Pick<Campaign, "send_at" | "send_in_user_timezone" | "state"> & {
    launch_type?: CampaignLaunchType
}
// export type ListUpdateParams = Pick<List, 'name' | 'rule'>
export type CampaignUser = User & { state: CampaignSendState; send_at: string }

interface NamedEmail {
    name: string
    address: string
}
export interface EmailTemplateData {
    from: NamedEmail
    cc?: string
    bcc?: string
    reply_to?: string
    subject: string
    preheader?: string
    editor: "code" | "visual"
    text: string
    html: string
}

export interface TextTemplateData {
    from: string
    text: string
}

export interface PushTemplateData {
    title: string
    body: string
    url: string
    custom: Record<string, unknown>
}

export type Template = {
    id: UUID
    campaign_id: UUID
    type: ChannelType
    locale: string
    data: any
    screenshot_url: string
    created_at: string
    updated_at: string
} & (
    | {
          type: "email"
          data: EmailTemplateData
      }
    | {
          type: "text"
          data: TextTemplateData
      }
    | {
          type: "push"
          data: PushTemplateData
      }
)

export type TemplateCreateParams = Pick<Template, "data" | "locale">
export type TemplateUpdateParams = Pick<Template, "data">
export type VariantUpdateParams = { id?: UUID }

export interface TemplatePreviewParams {
    user: Record<string, any>
    event: Record<string, any>
    ontext: Record<string, any>
}

export interface TemplateProofParams {
    variables: TemplatePreviewParams
    recipient: string
}

export type SubscriptionState = "subscribed" | "unsubscribed"

export interface UserSubscription {
    id: UUID
    name: string
    channel: ChannelType
    subscription_id: UUID
    state: SubscriptionState
    created_at: string
    updated_at: string
}

export interface SubscriptionParams {
    state: SubscriptionState
    subscription_id: UUID
}

export interface Subscription {
    id: UUID
    name: string
    channel: ChannelType
    is_public: boolean
    created_at: string
    updated_at: string
}
export type SubscriptionCreateParams = Pick<Subscription, "name" | "channel" | "is_public">
export type SubscriptionUpdateParams = Pick<SubscriptionCreateParams, "name" | "is_public">

export type ProviderGroup = "email" | "text" | "push"
export interface Provider {
    id: UUID
    name: string
    module: string
    channel: string

    data: any
    is_default: boolean
    setup: ProviderSetupMeta[]
    external_id?: string
}

export type ProviderCreateParams = Pick<Provider, "name" | "data" | "module" | "channel">
export type ProviderUpdateParams = ProviderCreateParams
export interface ProviderMeta {
    name: string
    description?: string
    url?: string
    icon?: string
    type: string
    group: string

    schema: any
    paths?: Record<string, string>
}

export interface ProviderSetupMeta {
    name: string
    value: string
}

export interface Image {
    id: UUID
    uuid: string
    url: string
    name: string
    original_name: string
    extension: string
    alt: string
    filesize: string
}

export interface Resource {
    id: UUID
    type: string
    name: string
    value: Record<string, any>
}

export interface Font {
    name: string
    url: string
    value: string
}

export interface Tag {
    id: UUID
    name: string
    count?: number
}

export interface LocaleOption {
    key: string
    label: string
}

export interface Locale extends LocaleOption {
    id: UUID
}

export type ActionType = "webhook"

export interface Action {
    id: UUID
    project_id: UUID
    name: string
    type: ActionType
    config: Record<string, any>
    created_at: string
    updated_at: string
}

export type ActionCreateParams = Pick<Action, "name" | "type"> & {
    config?: Record<string, any>
}

export type ActionUpdateParams = Partial<ActionCreateParams>
