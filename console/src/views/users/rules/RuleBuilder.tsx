import type { Rule, ControlledInputProps, FieldProps } from "../../../types"
import type { FieldPath, FieldValues } from "react-hook-form"
import type { ReactNode } from "react"
import { useController } from "react-hook-form"
import { useCallback, useContext, useMemo } from "react"
import { ProjectContext } from "../../../contexts"
import { useResolver } from "../../../hooks"
import { oapiClient } from "@/oapi/client"
import { snakeToTitle } from "../../../utils"
import { emptySuggestions, VariablesContext } from "./RuleHelpers"
import RuleEdit from "./RuleEdit"
import { Label } from "@/components/ui/label"

interface RuleBuilderParams {
    rule: Rule
    setRule: (rule: Rule) => void
    headerPrefix?: ReactNode
    eventName?: string
    userOnly?: boolean
    journeyContext?: boolean
}

export default function RuleBuilder({
    eventName,
    headerPrefix,
    rule,
    setRule,
    userOnly,
    journeyContext,
}: RuleBuilderParams) {
    const [{ id: projectId }] = useContext(ProjectContext)
    const [suggestions] = useResolver(
        useCallback(async () => {
            const [eventsRes, usersRes] = await Promise.all([
                oapiClient.GET('/api/admin/projects/{projectID}/events/schema', {
                    params: { path: { projectID: projectId } }
                }),
                oapiClient.GET('/api/admin/projects/{projectID}/users/schema', {
                    params: { path: { projectID: projectId } }
                })
            ])

            if (!eventsRes.data || !usersRes.data) {
                return emptySuggestions
            }

            const eventPaths = eventsRes.data.results.map(event => ({
                id: event.id,
                name: event.name,
                schema: (event.schema ?? []).map(schemaPath => ({
                    path: `.data${schemaPath.path}`,
                    types: schemaPath.types,
                }))
            }))

            return {
                eventPaths,
                userPaths: usersRes.data.results,
            }
        }, [projectId]),
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
