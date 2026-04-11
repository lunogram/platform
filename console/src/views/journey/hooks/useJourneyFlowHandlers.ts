import { useCallback, type Dispatch, type SetStateAction } from "react"
import { addEdge, type Connection, type Edge, type ReactFlowInstance } from "reactflow"
import { useTranslation } from "react-i18next"
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
    const { t } = useTranslation()

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

            const defaultData = type.newData ? await type.newData() : {}

            // If the dragged payload includes initial data (e.g. action_id from an
            // action card), merge it into the default step data.
            const data = payload.data ? { ...defaultData, ...payload.data } : defaultData

            // Auto-name entrance steps when another entrance already exists
            let name = payload.name as string | undefined
            if (payload.type === "entrance" && !name) {
                const entranceCount = nodes.filter((n) => n.data.type === "entrance").length
                if (entranceCount > 0) {
                    name = `${t("entrance")} ${entranceCount + 1}`
                }
            }

            const isSticky = payload.type === "sticky"
            const newNode: JourneyNode = {
                id: createUuid(),
                position: { x, y },
                type: "step",
                data: {
                    type: payload.type,
                    name,
                    data,
                    ...(isSticky ? { width: 275, height: 150 } : {}),
                },
                ...(isSticky ? { style: { width: 275, height: 150 } } : {}),
            }

            setHasUnsavedChanges(true)
            setNodes([...nodes, newNode])
            pushHistory()
        },
        [flowInstance, nodes, setNodes, setHasUnsavedChanges, wrapper, pushHistory, t],
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
