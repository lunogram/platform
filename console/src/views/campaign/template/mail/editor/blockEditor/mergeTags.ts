import type { VariableGroup } from "@/views/journey/JourneyVariableContext"

/** Templatical's merge tag shape, kept local so the module owns the mapping. */
export interface TemplaticalMergeTag {
    label: string
    value: string
    group?: string
    description?: string
}

/**
 * Map the campaign's variable groups onto Templatical's merge tag picker.
 *
 * The editor is configured with the `liquid` syntax preset, so a tag's `value`
 * is the same `{{ path }}` expression the send pipeline resolves — nothing
 * translates between the two. `label` is what the editor canvas displays in
 * place of the raw expression.
 *
 * Object and array variables are dropped: they have no scalar rendering, so
 * inserting one would emit `[object Object]` into the email.
 */
export function toMergeTags(groups: VariableGroup[]): TemplaticalMergeTag[] {
    return groups.flatMap((group) =>
        group.variables
            .filter((variable) => !isNonScalar(variable.types))
            .map((variable) => ({
                label: variable.label,
                value: `{{ ${variable.path} }}`,
                group: group.label,
                description: variable.description,
            })),
    )
}

function isNonScalar(types: string[] | undefined): boolean {
    if (!types?.length) return false
    return types.every((type) => type === "object" || type === "array")
}
