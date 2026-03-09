import type { Rule, ControlledInputProps, FieldProps } from "../../../types"
import type { FieldPath, FieldValues } from "react-hook-form"
import type { ReactNode } from "react"
import { useController } from "react-hook-form"
import { useCallback, useContext, useMemo } from "react"
import { ProjectContext } from "../../../contexts"
import { useResolver } from "../../../hooks"
import api from "../../../api"
import { snakeToTitle } from "../../../utils"
import { emptySuggestions, VariablesContext } from "./RuleHelpers"
import type { VariableGroup } from "@/views/journey/JourneyVariableContext"
import RuleEdit from "./RuleEdit"
import { Label } from "@/components/ui/label"

interface RuleBuilderParams {
    rule: Rule
    setRule: (rule: Rule) => void
    headerPrefix?: ReactNode
    eventName?: string
    userOnly?: boolean
    journeyContext?: boolean
    journeyVariables?: VariableGroup[]
}

export default function RuleBuilder({
    eventName,
    headerPrefix,
    rule,
    setRule,
    userOnly,
    journeyContext,
    journeyVariables,
}: RuleBuilderParams) {
    const [{ id: projectId }] = useContext(ProjectContext)
    const [suggestions] = useResolver(
        useCallback(async () => await api.projects.pathSuggestions(projectId), [projectId]),
    )
    return (
        <VariablesContext.Provider
            value={useMemo(() => ({ suggestions: suggestions ?? emptySuggestions }), [suggestions])}
        >
            <RuleEdit
                root={rule}
                rule={rule}
                setRule={setRule}
                group={eventName ? "event" : "parent"}
                eventName={eventName}
                headerPrefix={headerPrefix}
                userOnly={userOnly}
                journeyContext={journeyContext}
                journeyVariables={journeyVariables}
            />
        </VariablesContext.Provider>
    )
}

RuleBuilder.Field = function RuleBuilderField<X extends FieldValues, P extends FieldPath<X>>({
    form,
    name,
    label,
    required,
    onChange,
}: Partial<ControlledInputProps<Rule>> & FieldProps<X, P>) {
    const { field } = useController({
        control: form.control,
        name,
        rules: {
            required,
        },
    })

    return (
        <div className="space-y-2">
            <Label className="inline-flex items-center gap-1 text-sm font-medium">
                {label ?? snakeToTitle(name)}
                {required && <span className="text-destructive">*</span>}
            </Label>
            <RuleBuilder
                rule={field.value}
                setRule={async (rule) => {
                    await field.onChange?.(rule)
                    onChange?.(rule)
                }}
            />
        </div>
    )
}
