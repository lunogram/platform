import {
    createContext,
    useCallback,
    useContext,
    useEffect,
    useMemo,
    useState,
    type PropsWithChildren,
} from "react"
import type { Edge, Node } from "reactflow"
import type { JourneyNodeData } from "./editor/JourneyEditor.types"
import type { VariableSuggestions } from "@/types"
import { getUpstreamDataKeys } from "./editor/JourneyEditor.utils"
import { ProjectContext } from "@/contexts"
import api from "@/api"

// ── Public types ────────────────────────────────────────────────────

export interface Variable {
    /** Liquid-compatible path, e.g. "user.email" or "journey.my_key.amount" */
    path: string
    /** Human-readable label shown in the picker */
    label: string
    description?: string
}

export interface VariableGroup {
    label: string
    variables: Variable[]
}

// ── Context value ───────────────────────────────────────────────────

interface JourneyVariableContextValue {
    /** Get variable groups available for a specific node */
    getVariablesForNode: (nodeId: string) => VariableGroup[]
}

const JourneyVariableContext = createContext<JourneyVariableContextValue>({
    getVariablesForNode: () => [],
})

// eslint-disable-next-line react-refresh/only-export-components
export function useJourneyVariableContext() {
    return useContext(JourneyVariableContext)
}

// ── Provider ────────────────────────────────────────────────────────

interface JourneyVariableProviderProps {
    nodes: Node<JourneyNodeData>[]
    edges: Edge[]
}

export function JourneyVariableProvider({
    nodes,
    edges,
    children,
}: PropsWithChildren<JourneyVariableProviderProps>) {
    const [project] = useContext(ProjectContext)
    const [suggestions, setSuggestions] = useState<VariableSuggestions | null>(null)

    // Fetch user/event path suggestions once per project
    useEffect(() => {
        let cancelled = false
        api.projects
            .pathSuggestions(project.id)
            .then((s) => {
                if (!cancelled) setSuggestions(s)
            })
            .catch(console.error)
        return () => {
            cancelled = true
        }
    }, [project.id])

    const getVariablesForNode = useCallback(
        (nodeId: string): VariableGroup[] => {
            const groups: VariableGroup[] = []

            // ── User properties ─────────────────────────────────
            if (suggestions?.userPaths?.length) {
                groups.push({
                    label: "User",
                    variables: suggestions.userPaths.map((p) => ({
                        path: `user.${p.path}`,
                        label: p.path,
                        description: p.types.join(", "),
                    })),
                })
            }

            // ── Journey step data (upstream ancestors with data_key) ─
            const upstream = getUpstreamDataKeys(nodes, edges, nodeId)
            if (upstream.length) {
                for (const ancestor of upstream) {
                    const prefix = `journey.${ancestor.data_key}`

                    // Find the event schema fields that match this specific
                    // ancestor's event_name (not all events in the project).
                    const matchingEvent = ancestor.event_name
                        ? suggestions?.eventPaths?.find((evt) => evt.name === ancestor.event_name)
                        : undefined

                    const fieldVars: Variable[] =
                        matchingEvent?.schema?.map((field) => {
                            // API returns paths with a leading dot (e.g. ".data.amount"),
                            // strip it to avoid double-dot in the resulting path.
                            const cleanPath = field.path.replace(/^\./, "")
                            return {
                                path: `${prefix}.${cleanPath}`,
                                label: cleanPath,
                                description: field.types.join(", "),
                            }
                        }) ?? []

                    groups.push({
                        label: ancestor.name
                            ? `${ancestor.name} (${ancestor.data_key})`
                            : ancestor.data_key,
                        variables: [
                            {
                                path: prefix,
                                label: `${ancestor.data_key} (full object)`,
                                description: `Data from step "${ancestor.name || ancestor.data_key}"`,
                            },
                            ...fieldVars,
                        ],
                    })
                }
            }

            return groups
        },
        [nodes, edges, suggestions],
    )

    const value = useMemo<JourneyVariableContextValue>(
        () => ({ getVariablesForNode }),
        [getVariablesForNode],
    )

    return (
        <JourneyVariableContext.Provider value={value}>{children}</JourneyVariableContext.Provider>
    )
}
