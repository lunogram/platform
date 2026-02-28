import type { ReactNode } from 'react';
import { createContext } from 'react'
import type { EventRule, Operator, OrganizationEventRule, OrganizationRule, OrganizationUserMatch, Rule, RuleGroup, RuleType, VariableSuggestions, WrapperRule } from '../../../types'
import { createUuid } from '../../../utils'

export interface GroupedRule extends Omit<Rule, 'value'> {
    value?: string | string[]
}

export const trimPathDisplay = (path: string = '') => path.startsWith('.') ? path.substring(2) : path

export const isEventWrapper = (rule: Rule): rule is EventRule => {
    return rule?.group === 'event'
        && (rule?.path === '.name' || rule?.path === 'name')
}

export const isOrganizationEventWrapper = (rule: Rule): rule is OrganizationEventRule => {
    return rule?.group === 'organization_event'
        && (rule?.path === '.name' || rule?.path === 'name')
}

export const isOrganizationWrapper = (rule: Rule): rule is OrganizationRule => {
    return rule?.group === 'organization'
        && rule?.type === 'wrapper'
}

export const isWrapper = (rule: Rule | GroupedRule): rule is WrapperRule => {
    return rule?.type === 'wrapper'
        && (rule?.group === 'parent' || rule?.group === 'event' || rule?.group === 'organization_event' || rule?.group === 'organization')
}

export const createWrapperRule = (): WrapperRule => ({
    uuid: createUuid(),
    path: '',
    type: 'wrapper',
    group: 'parent',
    operator: 'and',
    children: [],
})

// Create an initial member condition for the wrapper
export const createInitialMemberCondition = (wrapperRule: WrapperRule): Rule => ({
    uuid: createUuid(),
    root_uuid: wrapperRule.uuid,
    parent_uuid: wrapperRule.uuid,
    path: '',
    type: 'string',
    group: 'user',
    value: '',
    operator: '=',
})

export const createEventRule = (parent?: Rule, value = ''): EventRule => {
    const base: EventRule = {
        uuid: createUuid(),
        path: '.name',
        type: 'wrapper',
        group: 'event',
        value,
        operator: 'and',
        children: [],
        frequency: {
            period: {
                type: 'rolling',
                unit: 'day',
                value: 30,
            },
            operator: '>=',
            count: 1,
        },
    }
    if (parent) {
        return {
            ...base,
            root_uuid: parent.root_uuid ?? parent.uuid,
            parent_uuid: parent.uuid,
        }
    }
    return base
}

export const createSimpleEventRule = (value = ''): WrapperRule => ({
    uuid: createUuid(),
    path: '',
    type: 'wrapper',
    group: 'event',
    value,
    operator: 'and',
    children: [],
})

export const createSimpleOrganizationEventRule = (value = '', user_match?: OrganizationUserMatch): OrganizationEventRule => ({
    uuid: createUuid(),
    path: '',
    type: 'wrapper',
    group: 'organization_event',
    value,
    operator: 'and',
    children: [],
    user_match: user_match ?? createDefaultUserMatch(),
})

export const createDefaultUserMatch = (): OrganizationUserMatch => ({
    type: 'all',
})

export const createOrganizationEventRule = (parent?: Rule, value = ''): OrganizationEventRule => {
    const base: OrganizationEventRule = {
        uuid: createUuid(),
        path: '.name',
        type: 'wrapper',
        group: 'organization_event',
        value,
        operator: 'and',
        children: [],
        frequency: {
            period: {
                type: 'rolling',
                unit: 'day',
                value: 30,
            },
            operator: '>=',
            count: 1,
        },
        user_match: createDefaultUserMatch(),
    }
    if (parent) {
        return {
            ...base,
            root_uuid: parent.root_uuid ?? parent.uuid,
            parent_uuid: parent.uuid,
        }
    }
    return base
}

export const createOrganizationRule = (parent?: Rule): OrganizationRule => {
    const base: OrganizationRule = {
        uuid: createUuid(),
        path: '',
        type: 'wrapper',
        group: 'organization',
        operator: 'and',
        children: [],
        user_match: createDefaultUserMatch(),
    }
    if (parent) {
        return {
            ...base,
            root_uuid: parent.root_uuid ?? parent.uuid,
            parent_uuid: parent.uuid,
        }
    }
    return base
}

export const emptySuggestions: VariableSuggestions = {
    userPaths: [],
    eventPaths: [],
    organizationUserPaths: [],
    organizationPaths: [],
}

export const VariablesContext = createContext<{
    suggestions: VariableSuggestions
}>({
    suggestions: emptySuggestions,
})

export const ruleTypes: Array<{
    key: RuleType
    label: string
}> = [
        { key: 'string', label: 'String' },
        { key: 'number', label: 'Number' },
        { key: 'boolean', label: 'Boolean' },
        { key: 'date', label: 'Date' },
        { key: 'array', label: 'Array' },
    ]

const baseOperators: OperatorOption[] = [
    { key: '=', label: 'equals' },
    { key: '!=', label: 'does not equal' },
    { key: 'is set', label: 'is set' },
    { key: 'is not set', label: 'is not set' },
]

interface OperatorOption {
    key: Operator
    label: string
}

export const operatorTypes: Record<RuleType, OperatorOption[]> = {
    string: [
        ...baseOperators,
        { key: 'empty', label: 'is empty' },
        { key: 'contains', label: 'contains' },
        { key: 'not contain', label: 'does not contain' },
        { key: 'starts with', label: 'starts with' },
        { key: 'not start with', label: 'does not start with' },
    ],
    number: [
        ...baseOperators,
        { key: '<', label: 'is less than' },
        { key: '<=', label: 'is less than or equal to' },
        { key: '>', label: 'is greater than' },
        { key: '>=', label: 'is greater than or equal to' },
    ],
    boolean: [
        { key: '=', label: 'is' },
        { key: '!=', label: 'is not' },
    ],
    date: [
        ...baseOperators,
        { key: '<', label: 'is before' },
        { key: '<=', label: 'is on or before' },
        { key: '>', label: 'is after' },
        { key: '>=', label: 'is on or after' },
        { key: 'is same day', label: 'is same day' },
    ],
    array: [
        ...baseOperators,
        { key: 'empty', label: 'is empty' },
        { key: 'contains', label: 'contains' },
    ],
    wrapper: [
        { key: 'or', label: 'any' },
        { key: 'and', label: 'all' },
    ],
}

export const frequencyOperators: OperatorOption[] = [
    { key: '=', label: 'Exactly' },
    { key: '<', label: 'Less than' },
    { key: '<=', label: 'Less than or equal to' },
    { key: '>', label: 'Greater than' },
    { key: '>=', label: 'Greater than or equal to' },
]

export const periodUnits: Array<{ key: 'minute' | 'hour' | 'day' | 'week' | 'month', label: string }> = [
    { key: 'minute', label: 'Minutes' },
    { key: 'hour', label: 'Hours' },
    { key: 'day', label: 'Days' },
    { key: 'week', label: 'Weeks' },
    { key: 'month', label: 'Months' },
]

export const userMatchTypes: Array<{ key: 'all' | 'conditions', label: string, description: string }> = [
    { key: 'all', label: 'All members', description: 'Include all users in matching organizations' },
    { key: 'conditions', label: 'Members matching conditions', description: 'Include members matching property conditions' },
]

export interface RuleEditProps<T extends Rule = Rule> {
    rule: T
    root: Rule
    setRule: (value: T) => void
    group: RuleGroup
    eventName?: string
    depth?: number
    controls?: ReactNode
    headerPrefix?: ReactNode
}
