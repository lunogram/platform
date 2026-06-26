import { useCallback, type Dispatch, type SetStateAction } from "react"
import { addEdge, type Connection, MarkerType, type ReactFlowInstance } from "@xyflow/react"
import { useTranslation } from "react-i18next"
import { createUuid } from "@/utils"
import { DATA_FORMAT, STEP_STYLE } from "./JourneyEditor.constants"
import {
    defaultEntranceDataKey,
    getStepType,
    isValidJourneyConnection,
} from "../editor/JourneyEditor.utils"
import type { JourneyEdge, JourneyNode } from "../editor/JourneyEditor.types"

export function useJourneyFlowHandlers(
    nodes: JourneyNode[],
    edges: JourneyEdge[],
    setNodes: Dispatch<SetStateAction<JourneyNode[]>>,
    setEdges: Dispatch<SetStateAction<JourneyEdge[]>>,
    flowInstance: ReactFlowInstance<JourneyNode, JourneyEdge> | null,
    setHasUnsavedChanges: (val: boolean) => void,
    pushHistory: (snapshot?: { nodes: JourneyNode[]; edges: JourneyEdge[] }) => void,
) {
    const { t } = useTranslation()

    const onConnect = useCallback(
        async (conn: Connection) => {
            const currentNodes = (flowInstance?.getNodes() as JourneyNode[] | undefined) ?? nodes
            const sourceNode = currentNodes.find((n) => n.id === conn.source)
            const stepType = sourceNode?.data.type
            const data = stepType ? ((await getStepType(stepType)?.newEdgeData?.()) ?? {}) : {}
            const currentEdges = (flowInstance?.getEdges() as JourneyEdge[] | undefined) ?? edges
            if (!isValidJourneyConnection(conn, currentNodes, currentEdges)) return

            pushHistory({ nodes: currentNodes, edges: currentEdges })
            setHasUnsavedChanges(true)
            setEdges((eds) =>
                isValidJourneyConnection(conn, currentNodes, eds)
                    ? addEdge(
                          {
                              ...conn,
                              type: STEP_STYLE,
                              data,
                              markerEnd: { type: MarkerType.ArrowClosed },
                          },
                          eds,
                      )
                    : eds,
            )
        },
        [flowInstance, nodes, edges, setEdges, setHasUnsavedChanges, pushHistory],
    )

    const onDrop = useCallback(
        async (event: React.DragEvent) => {
            event.preventDefault()
            if (!flowInstance) return
            const payload = JSON.parse(event.dataTransfer.getData(DATA_FORMAT))
            const type = getStepType(payload.type)
            if (!type) return

            const { x, y } = flowInstance.screenToFlowPosition({
                x: event.clientX - (payload.x ?? 0),
                y: event.clientY - (payload.y ?? 0),
            })

            const defaultData = type.newData ? await type.newData() : {}

            // If the dragged payload includes initial data (e.g. action_id from an
            // action card), merge it into the default step data.
            const data = payload.data ? { ...defaultData, ...payload.data } : defaultData

            const isSticky = payload.type === "sticky"
            setHasUnsavedChanges(true)
            pushHistory()
            setNodes((nds) => {
                let name = payload.name as string | undefined
                if (payload.type === "entrance" && !name) {
                    const entranceCount = nds.filter((n) => n.data.type === "entrance").length
                    if (entranceCount > 0) name = `${t("entrance")} ${entranceCount + 1}`
                }

                // Give entrances a default data_key so their trigger event data is
                // immediately referenceable downstream as `journey.<data_key>.data.*`.
                const data_key =
                    payload.type === "entrance"
                        ? defaultEntranceDataKey(
                              nds.map((n) => n.data.data_key).filter((k): k is string => !!k),
                          )
                        : undefined

                // An exit must target an entrance (required field). Default to the
                // first entrance so single-entrance journeys need no manual pick.
                let stepData = data
                if (
                    payload.type === "exit" &&
                    !(stepData as { entrance_uuid?: string }).entrance_uuid
                ) {
                    const firstEntrance = nds.find((n) => n.data.type === "entrance")
                    if (firstEntrance) stepData = { ...stepData, entrance_uuid: firstEntrance.id }
                }

                const newNode: JourneyNode = {
                    id: createUuid(),
                    position: { x, y },
                    type: "step",
                    data: {
                        type: payload.type,
                        name,
                        data_key,
                        data: stepData,
                        ...(isSticky ? { width: 275, height: 150 } : {}),
                    },
                    ...(isSticky ? { style: { width: 275, height: 150 } } : {}),
                }

                return [...nds, newNode]
            })
        },
        [flowInstance, setNodes, setHasUnsavedChanges, pushHistory, t],
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
