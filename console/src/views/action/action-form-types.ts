import * as z from "zod"

export const VARIABLE_TYPES = ["string", "bool", "int"] as const
export type VariableType = (typeof VARIABLE_TYPES)[number]

export const actionFormSchema = z.object({
    name: z.string().min(1, "Name is required"),
    config: z.record(z.string(), z.unknown()).optional(),
    payload: z.record(z.string(), z.unknown()).optional(),
    variables: z
        .array(
            z.object({
                name: z.string().min(1, "Variable name is required"),
                type: z.enum(VARIABLE_TYPES),
                default: z.string(),
            }),
        )
        .optional(),
})

export type ActionFormValues = z.infer<typeof actionFormSchema>

export type StoredVariable = { name: string; type: VariableType; default?: string }

/** Convert stored variables array to form field array */
export function storedToForm(
    stored?: StoredVariable[],
): { name: string; type: VariableType; default: string }[] {
    if (!stored || !Array.isArray(stored)) return []
    return stored.map((v) => ({
        name: v.name,
        type: v.type ?? "string",
        default: v.default ?? "",
    }))
}

/** Convert form variables to a Record<string, any> using provided values (for test/preview) */
export function variablesToMap(
    arr?: { name: string; type: VariableType; default: string }[],
    overrides?: Record<string, string>,
): Record<string, unknown> {
    if (!arr) return {}
    const result: Record<string, unknown> = {}
    for (const v of arr) {
        if (!v.name.trim()) continue
        const key = v.name.trim()
        const raw = overrides?.[key] ?? v.default
        switch (v.type) {
            case "int":
                result[key] = raw === "" ? 0 : Number(raw)
                break
            case "bool":
                result[key] = raw === "true" || raw === "1"
                break
            default:
                result[key] = raw
        }
    }
    return result
}
