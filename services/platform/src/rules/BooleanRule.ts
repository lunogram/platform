import { RuleCheck } from './RuleEngine'
import { queryPath, queryValue, whereQuery } from './RuleHelpers'

export default {
    check({ rule, value }) {
        const values = queryValue(value, rule, item => {
            if (typeof item === 'boolean') return item
            if (typeof item === 'string') return item === 'true'
            if (typeof item === 'number') return item === 1
            return false
        })
        const match = values.some(Boolean)
        return rule.operator === '!=' ? !match : match
    },
    query({ rule }) {
        const castValue = (value: any) => {
            if (typeof value === 'boolean') return value
            if (typeof value === 'string') return value === 'true'
            if (typeof value === 'number') return value === 1
            return false
        }
        const path = queryPath(rule)
        const value = castValue(rule.value)
        return whereQuery(path, '=', value)
    },
} satisfies RuleCheck
