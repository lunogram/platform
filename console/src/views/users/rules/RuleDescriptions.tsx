import type { ReactNode } from "react"
import type {
    Operator,
    OrganizationEventRule,
    OrganizationRule,
    Preferences,
    Rule,
} from "../../../types"
import type { GroupedRule } from "./RuleHelpers"
import {
    isEventWrapper,
    isOrganizationEventWrapper,
    isOrganizationWrapper,
    isStepVisitRule,
    isWrapper,
    operatorTypes,
    stepVisitOperators,
    stepVisitScopes,
    trimPathDisplay,
} from "./RuleHelpers"
import { formatDate } from "../../../utils"

export function ruleDescription(
    preferences: Preferences,
    rule: Rule | GroupedRule,
    nodes: ReactNode[] = [],
    wrapperOperator?: Operator,
): ReactNode {
    const root = nodes.length === 0

    if (isStepVisitRule(rule)) {
        const operator =
            stepVisitOperators.find((op) => op.key === rule.operator)?.label ?? rule.operator
        const scope = stepVisitScopes.find((s) => s.key === (rule.step_scope ?? "entry"))?.label

        nodes.push(<strong key={nodes.length}>{rule.path || "this step"}</strong>)
        nodes.push(` ${operator} `)
        nodes.push(<strong key={nodes.length}>{rule.value ?? ""}</strong>)
        nodes.push(scope ? ` times ${scope}` : " times")

        return root ? <span className="inline text-sm">{nodes}</span> : nodes
    }

    if (isWrapper(rule)) {
        if (isOrganizationEventWrapper(rule)) {
            const orgRule = rule as OrganizationEventRule
            if (!root) {
                nodes.push(
                    "organization has done ",
                    <strong key={nodes.length}>{orgRule.value ?? ""}</strong>,
                )
            } else {
                nodes.push(
                    "Organization event: ",
                    <strong key={nodes.length}>{orgRule.value ?? ""}</strong>,
                )
            }
            // Add user match description
            if (orgRule.user_match) {
                const matchType = orgRule.user_match.type
                if (matchType === "all") {
                    nodes.push(" (all members)")
                } else if (matchType === "conditions") {
                    nodes.push(" (members matching conditions)")
                }
            }
            if (orgRule.children?.length) {
                nodes.push(" where ")
            }
        } else if (isOrganizationWrapper(rule)) {
            const orgRule = rule as OrganizationRule
            nodes.push("Organizations where ")
            // Add user match description
            if (orgRule.user_match) {
                const matchType = orgRule.user_match.type
                if (matchType === "all") {
                    nodes.push("(all members) ")
                } else if (matchType === "conditions") {
                    nodes.push("(members matching conditions) ")
                }
            }
        } else if (isEventWrapper(rule)) {
            if (!root) {
                nodes.push("has user done ", <strong key={nodes.length}>{rule.value ?? ""}</strong>)
            } else {
                nodes.push(<strong key={nodes.length}>{rule.value ?? ""}</strong>)
            }
            if (rule.children?.length) {
                nodes.push(" where ")
            }
        }
        if (rule.children?.length) {
            const grouped: GroupedRule[] = []
            for (const child of rule.children) {
                if (child.type === "wrapper") {
                    grouped.push(child)
                    continue
                }
                const path = trimPathDisplay(child.path)
                const prev = grouped.find(
                    (g) => trimPathDisplay(g.path) === path && g.operator === child.operator,
                )
                if (prev) {
                    if (Array.isArray(prev.value)) {
                        prev.value.push(child.value ?? "")
                    } else {
                        prev.value = [prev.value ?? "", child.value ?? ""]
                    }
                } else {
                    grouped.push({ ...child }) // copy so we don't modify original
                }
            }
            grouped.forEach((g, i) => {
                if (i > 0) {
                    nodes.push(", ")
                    if (wrapperOperator) {
                        nodes.push(rule.operator === "and" ? "and " : "or ")
                    }
                }
                ruleDescription(preferences, g, nodes, rule.operator)
            })
        }
        if (isEventWrapper(rule) && rule.frequency) {
            nodes.push(` ${rule.frequency.operator} ${rule.frequency.count} times`)
        }
        if (isOrganizationEventWrapper(rule)) {
            const orgRule = rule as OrganizationEventRule
            if (orgRule.frequency) {
                nodes.push(` ${orgRule.frequency.operator} ${orgRule.frequency.count} times`)
            }
        }
    } else {
        if (rule.group === "event" && (rule.path === "$.name" || rule.path === "name")) {
            nodes.push("event ")
        }
        if (rule.group === "user") {
            nodes.push("user property ")
        }
        if (rule.group === "journey") {
            nodes.push("journey data ")
        }

        nodes.push(
            <code
                key={nodes.length}
                className="rounded bg-muted px-1 py-0.5 font-mono text-[0.9em]"
            >
                {trimPathDisplay(rule.path)}
            </code>,
        )

        nodes.push(
            " " +
                (operatorTypes[rule.type]?.find((ot) => ot.key === rule.operator)?.label ??
                    rule.operator),
        )

        if (
            rule.operator !== "empty" &&
            rule.operator !== "is set" &&
            rule.operator !== "is not set"
        ) {
            nodes.push(" ")
            const values = Array.isArray(rule.value) ? rule.value : [rule.value ?? ""]
            values.forEach((value, i, a) => {
                if (i > 0) {
                    nodes.push(", ")
                    if (i === a.length - 1 && wrapperOperator) {
                        nodes.push(wrapperOperator === "and" ? "and " : "or ")
                    }
                }
                if (value.includes("{{")) {
                    nodes.push(
                        <code
                            key={nodes.length}
                            className="rounded bg-muted px-1 py-0.5 font-mono text-[0.9em]"
                        >
                            {value}
                        </code>,
                    )
                } else {
                    value = value.trim()
                    if (rule.type === "boolean") value = "true"
                    if (rule.type === "number") {
                        if (value.includes(".")) {
                            value = parseFloat(value).toLocaleString()
                        } else {
                            value = parseInt(value, 10).toLocaleString()
                        }
                    }
                    if (rule.type === "date") {
                        value = formatDate(preferences, value, "Ppp")
                    }
                    nodes.push(<strong key={nodes.length}>{value}</strong>)
                }
            })
        }
    }
    if (root) {
        return <span className="inline text-sm">{nodes}</span>
    }
    return nodes
}
