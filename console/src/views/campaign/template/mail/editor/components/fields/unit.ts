export const addUnit = (value: string) => {
    if (/^(auto|none|inherit|initial)$/.test(value)) {
        return value
    }

    if (/^(\d+\.?\d*)(px|%)$/.test(value)) {
        return `[${value}]`
    }
    // If it's a pure number, append 'px'
    if (/^\d+\.?\d*$/.test(value)) {
        return `[${value}px]`
    }
    // Otherwise return as-is (for calc(), var(), etc.)
    return `[${value}]`
}

/**
 * Helper function to check if any properties from a given group exist in the config
 * @param config - The configuration object to check
 * @param properties - Array of property names to check for
 * @returns true if any of the properties exist in the config
 */
export const hasAnyProperty = (config: Record<string, unknown>, properties: string[]): boolean => {
    return properties.some((prop) => prop in config)
}

type ClassGenerator = (value: string, prefix: string) => string

/**
 * Collects all non-boolean property keys across all breakpoints
 */
function collectAllProperties<T>(breakouts: {
    sm?: Partial<T>
    md?: Partial<T>
    xl?: Partial<T>
}): Set<string> {
    const properties = new Set<string>()
    const breakpointOrder = ["sm", "md", "xl"] as const

    if (!breakouts) {
        return properties
    }

    for (const breakpoint of breakpointOrder) {
        const viewport = breakouts[breakpoint]
        if (!viewport) continue

        Object.keys(viewport).forEach((key) => {
            const value = viewport[key as keyof typeof viewport]
            if (typeof value !== "boolean") {
                properties.add(key)
            }
        })
    }

    return properties
}

/**
 * Collects values for a specific property across all breakpoints
 */
function collectPropertyValues<T>(
    property: string,
    breakouts: {
        sm?: Partial<T>
        md?: Partial<T>
        xl?: Partial<T>
    },
): Map<string, string> {
    const values = new Map<string, string>()
    const breakpointOrder = ["sm", "md", "xl"] as const

    for (const breakpoint of breakpointOrder) {
        const viewport = breakouts[breakpoint]
        if (!viewport) continue

        const value = viewport[property as keyof typeof viewport]
        if (typeof value === "string" && value) {
            values.set(breakpoint, value)
        }
    }

    return values
}

/**
 * Determines if a property has different values across breakpoints
 */
function hasMultipleValues(propertyValues: Map<string, string>): boolean {
    const uniqueValues = new Set(propertyValues.values())
    return uniqueValues.size > 1
}

/**
 * Generates Tailwind classes for a single property across all its breakpoints
 */
function generatePropertyClasses(
    property: string,
    propertyValues: Map<string, string>,
    propertyToClassMap: Record<string, ClassGenerator>,
): string[] {
    const classes: string[] = []
    const breakpointOrder = ["sm", "md", "xl"] as const
    const needsBreakpointPrefixes = hasMultipleValues(propertyValues)
    const firstBreakpoint = Array.from(propertyValues.keys())[0]

    for (const breakpoint of breakpointOrder) {
        const value = propertyValues.get(breakpoint)
        if (!value || !propertyToClassMap[property]) continue

        const isFirstOccurrence = breakpoint === firstBreakpoint
        const prefix = needsBreakpointPrefixes && !isFirstOccurrence ? `${breakpoint}:` : ""

        const classString = propertyToClassMap[property](value, prefix)
        classes.push(classString)
    }

    return classes
}

/**
 * Generates Tailwind CSS classes from responsive breakpoint configurations
 * Only applies breakpoint prefixes to properties that have different values across breakpoints
 */
export function generateTailwindClasses<T extends Record<string, string | undefined | boolean>>(
    breakouts: {
        sm?: Partial<T>
        md?: Partial<T>
        xl?: Partial<T>
    },
    propertyToClassMap: Record<string, ClassGenerator>,
): string {
    const allClasses: string[] = []
    const allProperties = collectAllProperties(breakouts)

    for (const property of allProperties) {
        const propertyValues = collectPropertyValues(property, breakouts)
        const propertyClasses = generatePropertyClasses(
            property,
            propertyValues,
            propertyToClassMap,
        )
        allClasses.push(...propertyClasses)
    }

    return allClasses.join(" ")
}
