import { Combobox } from "../../../components/ui/combobox"
import type { Rule, RulePath } from "../../../types"
import { highlightSearch, usePopperSelectDropdown } from "@/lib/ui-utils"
import { useContext } from "react"
import { VariablesContext } from "./RuleHelpers"

export default function RuleEventName<T extends Rule>({
    rule,
    setRule,
}: {
    rule: T
    setRule: (rule: T) => void
}) {
    usePopperSelectDropdown()

    const { suggestions } = useContext(VariablesContext)

    // Convert event names (keys) to RulePath objects for the Combobox

    const eventOptions: RulePath[] = [
        ...(Array.isArray(suggestions.eventPaths)
            ? suggestions.eventPaths.map((event, index) => ({
                  id: `event-${index}`,
                  name: event.name,
                  path: event.name,
                  type: "event" as const,
                  data_type: "string" as const,
                  visibility: "public" as const,
              }))
            : []),
        ...(Array.isArray(suggestions.scheduledPaths)
            ? suggestions.scheduledPaths.map((scheduled, index) => ({
                  id: `scheduled-${index}`,
                  name: `scheduled.${scheduled.name}`,
                  path: `scheduled.${scheduled.name}`,
                  type: "event" as const,
                  data_type: "string" as const,
                  visibility: "public" as const,
              }))
            : []),
    ]

    return (
        <Combobox
            value={rule.value ?? ""}
            onValueChange={(selectedPath: string) => {
                setRule({ ...rule, value: selectedPath })
            }}
            options={eventOptions}
            placeholder="Event name"
            ariaLabel="Event name"
            inputAriaLabel="Event name"
            required
            renderOption={(option, search) => highlightSearch(option.name, search)}
        />
    )
}
