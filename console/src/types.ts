import type { ComponentType, Dispatch, Key, ReactNode, SetStateAction } from "react"
import type { FieldPath, FieldValues, UseFormReturn } from "react-hook-form"
import type { Node } from "@xyflow/react"
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
    | "journey"
    | "journey_step"

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
    step_scope?: StepScope
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
    | {
          type: "since_entered"
      }

export interface EventRuleFrequency {
    period: EventRulePeriod
    operator: Operator
    count?: number
}

// Step Visit Rule - compares how often a user reached a journey step. An empty
// path refers to the step the rule is configured on.
export type StepScope = "entry" | "journey"

export type StepVisitRule = Rule & {
    group: "journey_step"
    type: "number"
    step_scope?: StepScope
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
    type: "user" | "event" | "scheduled"
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

export interface ScheduleOffset {
    id: UUID
    schedule_id: UUID
    offset: string
    direction: "before" | "after"
    created_at: string
    updated_at: string
}

export interface ScheduledSchema {
    id: UUID
    name: string
    schema: EventSchemaPath[]
    offsets?: ScheduleOffset[]
}

export interface ScheduledInstance {
    id: UUID
    user_id: UUID
    scheduled_id: UUID
    scheduled_at: string
    start_at: string | null
    interval: string | null
    data: Record<string, unknown> | null
    paused_at: string | null
    created_at: string
    updated_at: string
}

export interface VariableSuggestions {
    userPaths: UserSchemaPath[]
    eventPaths: EventSchema[]
    scheduledPaths?: ScheduledSchema[]
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
    include_deleted?: boolean
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

export interface SessionRefresh {
    expires_at: string
}

export const projectRoles = ["support", "client", "editor", "admin"] as const

export interface AdminOrganization {
    id: UUID
    name: string
    role: string
    is_active: boolean
}

export interface ProjectInvite {
    id: UUID
    project_id: UUID
    project_name?: string | null
    inviter_admin_id: UUID | null
    inviter_admin_email: string | null
    invitee_email: string
    invitee_admin_id: UUID | null
    role: ProjectRole
    expires_at: string
    accepted_at: string | null
    revoked_at: string | null
    created_at: string
}

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
    created_at: string
    updated_at: string
    deleted_at?: string
    role?: ProjectRole
    has_provider?: boolean
    campaigns_count?: number
    journeys_count?: number
    integrations_count?: number
    users_count?: number
    lists_count?: number
}

export type ChannelType = "email" | "push" | "sms" | "inbox"

export type ProjectCreate = Omit<Project, "id" | AuditFields>

export const authMethodTypes = ["api_key", "trusted_issuer", "session"] as const
export type AuthMethodType = (typeof authMethodTypes)[number]

// SubjectScope is an auth method's data boundary: "all" acts across every
// subject's records; "own" confines a verified end user to their own. Only
// meaningful for verified-subject types (trusted_issuer, session); api_key is
// always "all". Mirrors SubjectScope in the management OpenAPI schema.
export const subjectScopes = ["all", "own"] as const
export type SubjectScope = (typeof subjectScopes)[number]

export const grantVerbs = ["read", "create", "update", "delete"] as const
export type GrantVerb = (typeof grantVerbs)[number]

export interface PermissionGrant {
    resource: string
    verb: GrantVerb
}

// GrantConstraints narrows which named instances a client may create within a
// resource it already has create access to (e.g. specific event names). Keyed by
// resource name; a resource present with a non-empty list is restricted to those
// names, and an absent resource is unrestricted. (To deny creation entirely,
// don't grant create — there is no "allow nothing" state.)
export type GrantConstraints = Partial<Record<string, string[]>>

// grantableResources mirrors rbac.Resources() in internal/rbac/model.go and must
// stay in sync with it. It drives the custom-permission matrix in the Access UI.
// Client-facing resources are listed first so the common cases surface at the top.
export const grantableResources = [
    "users",
    "events",
    "inbox",
    "scheduled",
    "devices",
    "organizations",
    "subscriptions",
    "campaigns",
    "broadcasts",
    "journeys",
    "lists",
    "tags",
    "templates",
    "locales",
    "documents",
    "actions",
    "providers",
    "push_providers",
    "sender_identities",
] as const

export interface ClaimMapping {
    sub?: string
}

export interface TrustedIssuerConfig {
    jwks_url?: string
    public_cert?: string
    iss?: string
    aud?: string
    claim?: ClaimMapping
}

export interface SessionConfig {
    ttl_seconds?: number
}

export interface AuthMethod {
    id: UUID
    project_id: UUID
    type: AuthMethodType
    name: string
    description?: string
    role: ProjectRole
    subject_scope?: SubjectScope
    grants?: PermissionGrant[]
    grant_constraints?: GrantConstraints
    trusted_issuer?: TrustedIssuerConfig
    session?: SessionConfig
    // secret is present only in the response to creating an api_key method.
    secret?: string
    created_at: string
    updated_at: string
}

export interface CreateAuthMethodParams {
    type: AuthMethodType
    name: string
    description?: string
    role?: ProjectRole
    subject_scope?: SubjectScope
    grants?: PermissionGrant[]
    grant_constraints?: GrantConstraints
    trusted_issuer?: TrustedIssuerConfig
    session?: SessionConfig
}

export type UpdateAuthMethodParams = Partial<
    Pick<
        CreateAuthMethodParams,
        "name" | "description" | "role" | "subject_scope" | "grants" | "grant_constraints"
    >
>

export interface ExternalIDResponse {
    id: UUID
    source: string
    external_id: string
    metadata?: Record<string, unknown> | null
    created_at: string
    updated_at: string
}

export interface User {
    id: UUID
    identifier: ExternalIDResponse[]
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
    identifier: ExternalIDResponse[]
    name?: string
    data: Record<string, unknown>
    version: number
    created_at: string
    updated_at: string
}

export type SubjectOrganizationCreateParams = {
    identifier: { source: string; external_id: string; metadata?: Record<string, unknown> | null }[]
    name?: string
    data: Record<string, unknown>
}
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
    id: UUID
    device_id: string
    data: Record<string, unknown>
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
    rule?: WrapperRule | null
    draft_rule?: WrapperRule | null
    version_number?: number | null
    users_count: number
    tags?: string[]
    progress?: {
        complete: number
        total: number
    }
    is_visible: boolean
    archived?: boolean
    created_at: string
    updated_at: string
} & (
    | {
          type: "dynamic"
          rule: WrapperRule | null
          draft_rule: WrapperRule | null
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

// eslint-disable-next-line @typescript-eslint/no-explicit-any
export interface JourneyStep<T = any> {
    id: UUID
    type: string
    name: string
    data: T
    x: number
    y: number
}

export type JourneyStepParams = Omit<JourneyStep, "id">

// eslint-disable-next-line @typescript-eslint/no-explicit-any
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
    nodeId?: string // ReactFlow node ID (for variable context lookups)
    onSaveDraft?: () => Promise<void>
}

export interface JourneyStepTypeEdgeProps<T, E> extends ControlledProps<E> {
    stepData: T
    siblingData: E[] // does not include self
    journey: Journey
    project: Project
}

// eslint-disable-next-line @typescript-eslint/no-explicit-any
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

export interface CampaignDelivery {
    sent: number
    total: number
    opens: number
    clicks: number
}

export interface CampaignVariable {
    name: string
    default?: string
}

export interface Campaign {
    id: UUID
    project_id: UUID
    name: string
    channel: ChannelType
    delivery: CampaignDelivery
    subscription_id?: UUID
    subscription?: Subscription
    transactional?: boolean
    templates: Template[]
    variables: CampaignVariable[]
    created_at: string
    updated_at: string
}

export type CampaignSendState = "pending" | "sent" | "throttled" | "failed" | "bounced" | "aborted"

export type BroadcastState =
    | "scheduled"
    | "pending"
    | "sending"
    | "completed"
    | "failed"
    | "cancelled"

export interface Broadcast {
    id: UUID
    project_id: UUID
    campaign_id: UUID
    list_id: UUID
    list_name: string
    list_type: ListType
    state: BroadcastState
    total: number
    sent: number
    failed?: number
    error?: string
    created_at: string
    updated_at: string
    started_at?: string
    completed_at?: string
    scheduled_at?: string
    campaign?: Pick<Campaign, "id" | "name" | "channel">
}

export interface BroadcastUser {
    id: UUID
    user_id: UUID
    state: string
    failure_reason?: string | null
    sent_at?: string
    full_name?: string
    email?: string
    phone?: string
}

export type CampaignUpdateParams = Partial<
    Pick<Campaign, "name" | "subscription_id" | "transactional" | "variables">
>
export type CampaignCreateParams = Pick<
    Campaign,
    "name" | "channel" | "subscription_id" | "transactional"
>
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
    type?: "react-email"
    code?: {
        source: string
        bundle?: string
        bundle_hash?: string
    }
    plaintext?: {
        generated?: string
        custom?: string
    }
}

export interface TextTemplateData {
    body: string
}

export interface PushTemplateData {
    title: string
    body: string
    url: string
    custom: Record<string, unknown>
}

export interface InboxTemplateData {
    title: string
    body: string
}

export type Template<
    DataObjectType extends
        | EmailTemplateData
        | TextTemplateData
        | PushTemplateData
        | InboxTemplateData =
        | EmailTemplateData
        | TextTemplateData
        | PushTemplateData
        | InboxTemplateData,
> = {
    id: UUID
    campaign_id: UUID
    type: ChannelType
    locale: string
    sender_identity_id: UUID | null
    data: DataObjectType
    screenshot_url: string
    created_at: string
    updated_at: string
} & (
    | {
          type: "email"
          data: EmailTemplateData
      }
    | {
          type: "sms"
          data: TextTemplateData
      }
    | {
          type: "push"
          data: PushTemplateData
      }
    | {
          type: "inbox"
          data: InboxTemplateData
      }
)

export type TemplateCreateParams = Pick<Template, "data" | "locale">
export type TemplateUpdateParams = Pick<Template, "data" | "sender_identity_id">
export type VariantUpdateParams = { id?: UUID }

export interface TemplatePreviewParams {
    user: Record<string, unknown>
    event: Record<string, unknown>
    ontext: Record<string, unknown>
}

export interface TemplateProofParams {
    variables: TemplatePreviewParams
    recipient: string
}

export interface EmailTemplate {
    id: string
    label: string
    description?: string
    thumbnail?: string
    html?: string
    text?: string
    blocks?: Record<string, unknown>
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

export type ProviderGroup = "email" | "sms" | "push"

export interface Image {
    id: UUID
    project_id: UUID
    url: string
    name: string
    filename: string
    key: string
    content_type: string
    size_bytes: number
    created_at: string
    updated_at: string
}

export interface Resource {
    id: UUID
    type: string
    name: string
    value: Record<string, unknown>
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
    config: Record<string, unknown>
    created_at: string
    updated_at: string
}

export type ActionCreateParams = Pick<Action, "name" | "type"> & {
    config?: Record<string, unknown>
}

export type ActionUpdateParams = Partial<ActionCreateParams>
