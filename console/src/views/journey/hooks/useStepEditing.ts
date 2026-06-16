import { useCallback, useMemo } from "react"
import type { JourneyEdge, JourneyNode } from "../editor/JourneyEditor.types"

export function useStepEditing(
    nodes: JourneyNode[],
    setNodes: React.Dispatch<React.SetStateAction<JourneyNode[]>>,
    setEdges: React.Dispatch<React.SetStateAction<JourneyEdge[]>>,
    setHasUnsavedChanges: (val: boolean) => void,
) {
    const editNode = useMemo(() => nodes.find((n) => n.data.editing), [nodes])
    const selectedNodes = useMemo(() => nodes.filter((n) => n.selected), [nodes])

    const updateNodes = useCallback(
        (nds: JourneyNode[]) => {
            setHasUnsavedChanges(true)
            setNodes(nds)
        },
        [setHasUnsavedChanges, setNodes],
    )

    const updateEditNode = useCallback(
        (partialData: Partial<JourneyNode["data"]>) => {
            if (!editNode) return
            updateNodes(
                nodes.map((n) =>
                    n.id === editNode.id ? { ...n, data: { ...n.data, ...partialData } } : n,
                ),
            )
        },
        [editNode, nodes, updateNodes],
    )

    const deleteNode = useCallback(
        (id: string) => {
            updateNodes(nodes.filter((item) => item.id !== id))
            setEdges((eds) => eds.filter((edge) => edge.source !== id && edge.target !== id))
        },
        [nodes, setEdges, updateNodes],
    )

    const stopEditing = useCallback(() => {
        setNodes((nds) => nds.map((n) => ({ ...n, data: { ...n.data, editing: false } })))
    }, [setNodes])

    return {
        editNode,
        selected: selectedNodes,
        updateEditNode,
        deleteNode,
        stopEditing,
        updateNodes,
    }
}
