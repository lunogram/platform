import { Combobox } from "../../../components/ui/combobox"
import type { OrganizationEventRule, RulePath } from "../../../types"
import { highlightSearch, usePopperSelectDropdown } from "@/lib/ui-utils"
import { useContext } from "react"
import { VariablesContext } from "./RuleHelpers"

export default function RuleOrganizationEventName({
    rule,
    setRule,
}: {
    rule: OrganizationEventRule
    setRule: (rule: OrganizationEventRule) => void
}) {
    usePopperSelectDropdown()

    const { suggestions } = useContext(VariablesContext)

    // Convert organization event names to RulePath objects for the Combobox
    // Falls back to regular event paths if organization-specific ones aren't available
    const eventSource = suggestions.organizationEventPaths ?? suggestions.eventPaths

    const eventOptions: RulePath[] = Array.isArray(eventSource)
        ? eventSource.map((event, index) => ({
              id: `org-event-${index}`,
              name: event.name,
              path: event.name,
              type: "event" as const,
              data_type: "string" as const,
              visibility: "public" as const,
          }))
        : []

    return (
        <Combobox
            value={rule.value ?? ""}
            onValueChange={(selectedPath: string) => {
                setRule({ ...rule, value: selectedPath })
            }}
            options={eventOptions}
            placeholder="Organization event name"
            ariaLabel="Organization event name"
            inputAriaLabel="Organization event name"
            required
            renderOption={(option, search) => highlightSearch(option.name, search)}
        />
    )
}
