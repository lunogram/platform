import type { Variable, VariableGroup } from "@/views/journey/JourneyVariableContext"
import type { User } from "@/types"

// Builds a preview props object from variable groups for use in the frontend
// email editor. Props are passed directly to the React component, so preview
// values should be realistic samples rather than Liquid placeholders.

/**
 * Set a nested property on an object by dot-separated path.
 * Creates intermediate objects as needed.
 */
function setNestedValue(obj: Record<string, unknown>, dotPath: string, value: unknown) {
    const parts = dotPath.split(".")
    let current: Record<string, unknown> = obj
    for (let i = 0; i < parts.length - 1; i++) {
        if (!(parts[i] in current) || typeof current[parts[i]] !== "object") {
            current[parts[i]] = {}
        }
        current = current[parts[i]] as Record<string, unknown>
    }
    current[parts[parts.length - 1]] = value
}

/**
 * Return a sample preview value for a variable based on its type and path.
 *
 * The preview renders client-side via React — props are passed directly to
 * the component, so values must be the right JS type (not Liquid templates).
 */
function sampleValue(variable: Variable): unknown {
    if (variable.defaultValue != null) return variable.defaultValue

    const type = variable.types?.[0]
    switch (type) {
        case "number":
            return 0
        case "boolean":
            return false
        case "array":
            return []
        case "object":
            return {}
        case "null":
            return null
        default:
            // "string", "date", or untyped — empty string by default.
            // Specific paths override this in PREVIEW_OVERRIDES below.
            return ""
    }
}

/**
 * Hard-coded preview overrides for specific variable paths.
 *
 * Most variables use the generic `sampleValue()`, but a few system
 * variables benefit from realistic placeholder values so the preview
 * looks meaningful.
 */
const PREVIEW_OVERRIDES: Record<string, unknown> = {
    unsubscribe_url: "https://lunogram.com/unsubscribe",
    preferences_url: "https://lunogram.com/preferences",
    now: new Date().toISOString(),
}

/**
 * Build a preview props object from variable groups.
 *
 * Each variable path becomes a nested property with a sample value
 * matching its schema type. For example:
 *   - "user.email" (string)  → { user: { email: "" } }
 *   - "user.data.age" (number) → { user: { data: { age: 0 } } }
 *   - "unsubscribe_url" (string) → { unsubscribe_url: "https://lunogram.com/unsubscribe" }
 *
 * These sample values are visible in the preview to indicate where
 * dynamic content will appear at send time.
 */
export function buildPreviewProps(variableGroups: VariableGroup[]): Record<string, unknown> {
    const props: Record<string, unknown> = {}

    for (const group of variableGroups) {
        for (const variable of group.variables) {
            const { path } = variable
            const cleanPath = path.split(" ")[0] // "now | date" → "now"

            // Skip variables whose clean path is already set (e.g. "now"
            // may appear multiple times with different Liquid filters)
            if (
                cleanPath in props ||
                (cleanPath.includes(".") && hasNestedValue(props, cleanPath))
            ) {
                continue
            }

            const value =
                cleanPath in PREVIEW_OVERRIDES
                    ? PREVIEW_OVERRIDES[cleanPath]
                    : sampleValue(variable)

            if (cleanPath.includes(".")) {
                setNestedValue(props, cleanPath, value)
            } else {
                props[cleanPath] = value
            }
        }
    }

    return props
}

/**
 * Check whether a dot-separated path already exists in a nested object.
 */
function hasNestedValue(obj: Record<string, unknown>, dotPath: string): boolean {
    const parts = dotPath.split(".")
    let current: unknown = obj
    for (const part of parts) {
        if (current == null || typeof current !== "object") return false
        current = (current as Record<string, unknown>)[part]
    }
    return current !== undefined
}

/**
 * Merge a real user's data into an existing preview props object.
 *
 * Populates the `user.*` subtree with the user's actual field values
 * (id, email, phone, etc.) and spreads `user.data` into the `user.data`
 * subtree. Non-user props (campaign.*, unsubscribe_url, etc.) are left
 * untouched.
 */
export function mergeUserIntoProps(
    baseProps: Record<string, unknown>,
    user: User,
): Record<string, unknown> {
    const props = structuredClone(baseProps)

    const userObj: Record<string, unknown> = {
        ...(typeof props.user === "object" && props.user !== null ? props.user : {}),
        id: user.id ?? "",
        email: user.email ?? "",
        phone: user.phone ?? "",
        external_id: user.external_id ?? "",
        anonymous_id: user.anonymous_id ?? "",
        timezone: user.timezone ?? "",
        locale: user.locale ?? "",
        created_at: user.created_at ? String(user.created_at) : "",
    }

    // Merge user.data — keep any schema-defined keys from base props,
    // then overlay with the real user's data values
    if (typeof userObj.data === "object" && userObj.data !== null) {
        userObj.data = { ...(userObj.data as Record<string, unknown>), ...user.data }
    } else {
        userObj.data = { ...user.data }
    }

    props.user = userObj
    return props
}

/**
 * Collect all known property paths from variable groups as a Set.
 *
 * Each variable contributes its cleaned top-level key and, for nested
 * paths, each intermediate segment (e.g. "user.email" adds "user" and
 * "user.email"). This lets callers compare the keys in a user-edited
 * props object against the schema to detect unknown properties.
 */
export function buildSchemaPaths(variableGroups: VariableGroup[]): Set<string> {
    const paths = new Set<string>()
    for (const group of variableGroups) {
        for (const variable of group.variables) {
            const parts = variable.path.split(" ")[0].split(".")
            let current = ""
            for (const part of parts) {
                current = current ? `${current}.${part}` : part
                paths.add(current)
            }
        }
    }
    return paths
}

/**
 * Find property paths in `obj` that are not present in `schemaPaths`.
 *
 * Walks the object recursively, building dot-separated paths, and
 * returns paths that don't appear in the schema. Only object values
 * are descended into — arrays and primitives are treated as leaves.
 */
export function findExtraProps(
    obj: Record<string, unknown>,
    schemaPaths: Set<string>,
    prefix = "",
): string[] {
    const extras: string[] = []
    for (const key of Object.keys(obj)) {
        const path = prefix ? `${prefix}.${key}` : key
        if (!schemaPaths.has(path)) {
            extras.push(path)
        } else {
            const val = obj[key]
            if (val && typeof val === "object" && !Array.isArray(val)) {
                extras.push(...findExtraProps(val as Record<string, unknown>, schemaPaths, path))
            }
        }
    }
    return extras
}

/**
 * A tree node used to build nested TypeScript type declarations
 * from flat variable paths.
 */
interface TypeTreeNode {
    children: Map<string, TypeTreeNode>
    /** Schema types from the backend (e.g. ["string"], ["number", "string"]). */
    types?: string[]
}

/**
 * Map backend schema types to TypeScript type expressions.
 *
 * Backend types (from `rules.JSONType` and hard-coded user columns):
 *   string, number, boolean, object, array, null, date
 *
 * When a variable has multiple observed types (e.g. ["number", "string"])
 * we emit a union.  Unknown types fall back to `string`.
 */
function schemaTypesToTs(types: string[]): string {
    const tsTypes = new Set<string>()
    for (const t of types) {
        switch (t) {
            case "number":
                tsTypes.add("number")
                break
            case "boolean":
                tsTypes.add("boolean")
                break
            case "object":
                tsTypes.add("Record<string, unknown>")
                break
            case "array":
                tsTypes.add("unknown[]")
                break
            case "null":
                tsTypes.add("null")
                break
            case "string":
            case "date": // dates are serialized as ISO strings
            default:
                tsTypes.add("string")
                break
        }
    }
    if (tsTypes.size === 0) return "string"
    return [...tsTypes].join(" | ")
}

/**
 * Generate TypeScript type declarations for the email template props.
 *
 * This creates an EmailProps interface that the user can reference in their
 * template: `export default function Email(props: EmailProps) { ... }`
 *
 * The function builds a proper nested type tree so that deeply nested
 * paths like `user.data.preferences.notifications` produce nested
 * interfaces rather than flat `string` properties.
 *
 * Leaf types are derived from `variable.types` when available,
 * falling back to `string` when type information is absent.
 */
export function generatePropsTypeDeclarations(variableGroups: VariableGroup[]): string {
    const root = new Map<string, TypeTreeNode>()

    for (const group of variableGroups) {
        for (const variable of group.variables) {
            const parts = variable.path.split(".")
            const topLevel = parts[0].split(" ")[0] // handle "now | date" style paths

            if (!root.has(topLevel)) {
                root.set(topLevel, { children: new Map() })
            }

            let leaf = root.get(topLevel)!

            if (parts.length > 1) {
                for (let i = 1; i < parts.length; i++) {
                    const part = parts[i]
                    if (!leaf.children.has(part)) {
                        leaf.children.set(part, { children: new Map() })
                    }
                    leaf = leaf.children.get(part)!
                }
            }

            // Store type information on the leaf node
            if (variable.types?.length) {
                leaf.types = variable.types
            }
        }
    }

    const interfaces: string[] = []
    const props: string[] = []

    function leafType(node: TypeTreeNode): string {
        return node.types?.length ? schemaTypesToTs(node.types) : "string"
    }

    function emitInterface(name: string, node: TypeTreeNode): string {
        const interfaceName = `${name.charAt(0).toUpperCase()}${name.slice(1)}Props`
        const fields: string[] = []

        for (const [key, child] of node.children) {
            if (child.children.size > 0) {
                const childInterface = emitInterface(`${name}_${key}`, child)
                fields.push(`readonly ${key}: ${childInterface};`)
            } else {
                fields.push(`readonly ${key}: ${leafType(child)};`)
            }
        }

        interfaces.push(`interface ${interfaceName} {\n    ${fields.join("\n    ")}\n}`)
        return interfaceName
    }

    for (const [topLevel, node] of root) {
        if (node.children.size > 0) {
            const interfaceName = emitInterface(topLevel, node)
            props.push(`readonly ${topLevel}: ${interfaceName};`)
        } else {
            props.push(`readonly ${topLevel}: ${leafType(node)};`)
        }
    }

    // Build the global EmailProps interface
    const propsInterface = `interface EmailProps {\n    ${props.join("\n    ")}\n}`

    return `${interfaces.join("\n\n")}\n\n${propsInterface}`
}
