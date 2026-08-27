import {
    createContext,
    useCallback,
    useContext,
    useEffect,
    useMemo,
    useState,
    type PropsWithChildren,
} from "react"
import type { Edge, Node } from "@xyflow/react"
import type { JourneyNodeData } from "./editor/JourneyEditor.types"
import type { VariableSuggestions } from "@/types"
import { getUpstreamDataKeys } from "./editor/JourneyEditor.utils"
import { ProjectContext } from "@/contexts"
import { fetchPathSuggestions } from "@/lib/path-suggestions"
import { snakeToTitle } from "@/utils"

export interface Variable {
    /** Liquid-compatible path, e.g. "user.email" or "journey.my_key.amount" */
    path: string
    /** Human-readable label shown in the picker */
    label: string
    description?: string
    /** Schema types from the backend, e.g. ["string"], ["number", "string"], ["object"], ["array"] */
    types?: string[]
    /** Default value defined by the campaign/journey variable, used as the preview sample */
    defaultValue?: unknown
}

export interface VariableGroup {
    label: string
    variables: Variable[]
}

export interface JourneyStepOption {
    /** Step external id, which is also the flow node id */
    id: string
    label: string
    type: string
}

interface JourneyVariableContextValue {
    /** Get variable groups available for a specific node */
    getVariablesForNode: (nodeId: string) => VariableGroup[]
    /** Every step on the canvas, for conditions that reference another step */
    steps: JourneyStepOption[]
}

const JourneyVariableContext = createContext<JourneyVariableContextValue>({
    getVariablesForNode: () => [],
    steps: [],
})

// eslint-disable-next-line react-refresh/only-export-components
export function useJourneyVariableContext() {
    return useContext(JourneyVariableContext)
}

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
        fetchPathSuggestions(project.id)
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

            if (suggestions?.userPaths?.length) {
                groups.push({
                    label: "User",
                    variables: suggestions.userPaths.map((p) => {
                        const cleanPath = p.path.replace(/^\./, "")
                        return {
                            path: `user.${cleanPath}`,
                            label: cleanPath,
                            description: p.types.join(", "),
                            types: p.types,
                        }
                    }),
                })
            }

            const upstream = getUpstreamDataKeys(nodes, edges, nodeId)
            if (upstream.length) {
                for (const ancestor of upstream) {
                    const prefix = `journey.${ancestor.data_key}`

                    // Find the event schema fields that match this specific
                    // ancestor's event_name (not all events in the project).
                    const matchingEvent = ancestor.event_name
                        ? suggestions?.eventPaths?.find((evt) => evt.name === ancestor.event_name)
                        : undefined

                    // Match scheduled schema (if the entrance trigger is "scheduled")
                    const matchingScheduled = ancestor.scheduled_name
                        ? suggestions?.scheduledPaths?.find(
                              (s) => s.name === ancestor.scheduled_name,
                          )
                        : undefined

                    const schemaSource = matchingEvent ?? matchingScheduled

                    const fieldVars: Variable[] =
                        schemaSource?.schema?.map((field) => {
                            // API returns paths with a leading dot (e.g. ".data.amount"),
                            // strip it to avoid double-dot in the resulting path.
                            const cleanPath = field.path.replace(/^\./, "")
                            return {
                                path: `${prefix}.${cleanPath}`,
                                label: cleanPath,
                                description: field.types.join(", "),
                                types: field.types,
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
                                types: ["object"],
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

    const steps = useMemo<JourneyStepOption[]>(
        () =>
            nodes.map(({ id, data }) => ({
                id,
                label: data.name?.trim() || snakeToTitle(data.type),
                type: data.type,
            })),
        [nodes],
    )

    const value = useMemo<JourneyVariableContextValue>(
        () => ({ getVariablesForNode, steps }),
        [getVariablesForNode, steps],
    )

    return (
        <JourneyVariableContext.Provider value={value}>{children}</JourneyVariableContext.Provider>
    )
}
