import type { VariableGroup } from "@/views/journey/JourneyVariableContext"

/** Templatical's display condition shape, kept local so the module owns the mapping. */
export interface TemplaticalDisplayCondition {
    label: string
    before: string
    after: string
    group?: string
    description?: string
}

/**
 * Offer each boolean variable as a pair of display conditions for the editor.
 *
 * Templatical does not model conditions as field/operator/value — a condition is
 * an opaque `before`/`after` markup pair the renderer wraps a block in, and the
 * platform owns whatever is inside it. So the picker's contents are entirely
 * ours to define, and a boolean attribute maps onto it directly: one condition
 * to show a block when the flag is set, one to show it when it is not.
 *
 * The emitted Liquid is the same dialect the send pipeline runs
 * (`osteele/liquid` server-side), so a condition authored here needs no
 * translation on the way out. The renderer wraps the block's markup in
 * `<mj-raw>` on both sides, which survives MJML compilation intact and leaves
 * the guards balanced whether the block sits at the top level or inside a
 * section column.
 *
 * Only variables whose sole schema type is boolean are offered. A path that the
 * backend has seen hold more than one type is not reliably a flag, and while
 * Liquid truthiness would still evaluate it, "is not true" on a field that is
 * sometimes a string means something the label does not say.
 */
export function toDisplayConditions(groups: VariableGroup[]): TemplaticalDisplayCondition[] {
    return groups.flatMap((group) =>
        group.variables
            .filter((variable) => isBoolean(variable.types))
            .flatMap((variable) => [
                {
                    // "is true" rather than "is set": the block shows only for a
                    // true value, where "set" would just as readily be read as
                    // "has a value", which an explicit false also satisfies.
                    label: `${variable.label} is true`,
                    before: `{% if ${variable.path} %}`,
                    after: "{% endif %}",
                    group: group.label,
                    description: `Only shown when ${variable.path} is true`,
                },
                {
                    // `unless` rather than `if … == false` so the condition also
                    // covers recipients the attribute was never set on. Those users
                    // are indistinguishable from an explicit false to the reader of
                    // the email, and treating them as such is what makes the pair
                    // exhaustive — every recipient matches exactly one of the two.
                    // "is not true" is the label that admits both, where "is false"
                    // would claim an explicit value the recipient may not have.
                    label: `${variable.label} is not true`,
                    before: `{% unless ${variable.path} %}`,
                    after: "{% endunless %}",
                    group: group.label,
                    description: `Only shown when ${variable.path} is false or not set`,
                },
            ]),
    )
}

function isBoolean(types: string[] | undefined): boolean {
    return types?.length === 1 && types[0] === "boolean"
}
