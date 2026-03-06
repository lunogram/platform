import { useCallback, type Dispatch, type SetStateAction } from "react"
import { addEdge, type Connection, type Edge, type ReactFlowInstance } from "reactflow"
import { createUuid } from "@/utils"
import { DATA_FORMAT, STEP_STYLE } from "./JourneyEditor.constants"
import { getStepType } from "../editor/JourneyEditor.utils"
import type { JourneyNode } from "../editor/JourneyEditor.types"

export function useJourneyFlowHandlers(
    nodes: JourneyNode[],
    setNodes: (nds: JourneyNode[]) => void,
    setEdges: Dispatch<SetStateAction<Edge[]>>,
    flowInstance: ReactFlowInstance | null,
    wrapper: React.RefObject<HTMLDivElement | null>,
    setHasUnsavedChanges: (val: boolean) => void,
    pushHistory: () => void,
) {
    const onConnect = useCallback(
        async (conn: Connection) => {
            const sourceNode = nodes.find((n) => n.id === conn.source)
            const stepType = sourceNode?.data.type
            const data = stepType ? ((await getStepType(stepType)?.newEdgeData?.()) ?? {}) : {}
            setHasUnsavedChanges(true)
            setEdges((eds: Edge[]) => addEdge({ ...conn, type: STEP_STYLE, data }, eds))
            pushHistory()
        },
        [nodes, setEdges, setHasUnsavedChanges, pushHistory],
    )

    const onDrop = useCallback(
        async (event: React.DragEvent) => {
            event.preventDefault()
            if (!wrapper.current || !flowInstance) return
            const bounds = wrapper.current.getBoundingClientRect()
            const payload = JSON.parse(event.dataTransfer.getData(DATA_FORMAT))
            const type = getStepType(payload.type)
            if (!type) return

            const { x, y } = flowInstance.project({
                x: event.clientX - bounds.left - (payload.x ?? 0),
                y: event.clientY - bounds.top - (payload.y ?? 0),
            })

            const newNode: JourneyNode = {
                id: createUuid(),
                position: { x, y },
                type: "step",
                data: {
                    type: payload.type,
                    data: type.newData ? await type.newData() : {},
                },
            }

            setHasUnsavedChanges(true)
            setNodes([...nodes, newNode])
            pushHistory()
        },
        [flowInstance, nodes, setNodes, setHasUnsavedChanges, wrapper, pushHistory],
    )

    const onNodeDoubleClick = useCallback(
        (_: unknown, n: JourneyNode) => {
            setNodes(
                nodes.map((x) => ({
                    ...x,
                    data: { ...x.data, editing: x.id === n.id },
                })),
            )
            setTimeout(
                () => flowInstance?.setCenter(n.position.x + 60, n.position.y + 60, { zoom: 1 }),
                50,
            )
        },
        [flowInstance, nodes, setNodes],
    )

    return { onConnect, onDrop, onNodeDoubleClick }
}
