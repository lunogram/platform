import { RuleCheck, RuleEvalException } from './RuleEngine'
import { queryPath, queryValue, whereQuery, whereQueryNullable } from './RuleHelpers'

export default {
    check({ rule, value }) {
        const values = queryValue(value, rule, item => item)

        if (rule.operator === 'is set') {
            return values.some(x => Array.isArray(x))
        }

        if (rule.operator === 'is not set') {
            return values.every(x => !Array.isArray(x))
        }

        if (rule.operator === 'empty') {
            return values.every(x => !Array.isArray(x) || x.length === 0)
        }

        throw new RuleEvalException(rule, 'unknown operator: ' + rule.operator)
    },
    query({ rule }) {
        const path = queryPath(rule)

        if (rule.operator === 'is set') {
            return whereQueryNullable(path, false)
        }

        if (rule.operator === 'is not set') {
            return whereQueryNullable(path, true)
        }

        if (rule.operator === 'empty') {
            return whereQuery(path, '=', [])
        }

        throw new RuleEvalException(rule, 'unknown operator: ' + rule.operator)
    },
} satisfies RuleCheck
