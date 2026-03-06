import { useEffect, useCallback, useRef, type SetStateAction, type Dispatch } from "react";
import { type Edge } from "reactflow";
import { cloneNodes } from "../editor/JourneyEditor.utils";
import type { JourneyNode } from "../editor/JourneyEditor.types";

interface ShortcutProps {
    nodes: JourneyNode[];
    edges: Edge[];
    setNodes: Dispatch<SetStateAction<JourneyNode[]>>;
    setEdges: Dispatch<SetStateAction<Edge[]>>;
    onNodesUpdated: () => void;
    enabled: boolean;
}

export function useKeyboardShortcuts({
    nodes,
    edges,
    setNodes,
    setEdges,
    onNodesUpdated,
    enabled,
}: ShortcutProps) {
    const clipboard = useRef<{ nodes: JourneyNode[]; edges: Edge[] } | null>(null);
    const history = useRef<{ nodes: JourneyNode[], edges: Edge[] }[]>([]);

    const pushHistory = useCallback(() => {
        history.current.push({ nodes, edges });
    }, [nodes, edges]);

    const undo = useCallback(() => {
        const prev = history.current.pop();
        if (prev) {
            setNodes(prev.nodes);
            setEdges(prev.edges);
        }
    }, [setNodes, setEdges]);

    const copy = useCallback(() => {
        const selectedNodes = nodes.filter((n) => n.selected);
        if (selectedNodes.length === 0) return;

        clipboard.current = {
            nodes: selectedNodes,
            edges: edges,
        };
    }, [nodes, edges]);

    const paste = useCallback(() => {
        if (!clipboard.current) return;

        onNodesUpdated();

        const { nodeCopies, edgeCopies } = cloneNodes(
            clipboard.current.edges,
            clipboard.current.nodes
        );

        setNodes([
            ...nodes.map((n) => ({ ...n, selected: false })),
            ...nodeCopies.map((n) => ({ ...n, selected: true })),
        ]);

        setEdges((eds: Edge[]) => [
            ...eds.map((e) => ({ ...e, selected: false })),
            ...edgeCopies,
        ]);

        clipboard.current.nodes = nodeCopies;
    }, [nodes, setNodes, setEdges, onNodesUpdated]);

    const duplicate = useCallback((e: KeyboardEvent) => {
        e.preventDefault();
        const selectedNodes = nodes.filter((n) => n.selected);
        if (selectedNodes.length === 0) return;

        onNodesUpdated();
        const { nodeCopies, edgeCopies } = cloneNodes(edges, selectedNodes);

        setNodes([
            ...nodes.map((n) => ({ ...n, selected: false })),
            ...nodeCopies.map((n) => ({ ...n, selected: true })),
        ]);

        setEdges((eds: Edge[]) => [
            ...eds.map((e) => ({ ...e, selected: false })),
            ...edgeCopies,
        ]);
    }, [nodes, edges, setNodes, setEdges, onNodesUpdated]);

    useEffect(() => {
        if (!enabled) return;

        const handleKeyDown = (e: KeyboardEvent) => {
            const target = e.target as HTMLElement;
            if (target.tagName === "INPUT" || target.tagName === "TEXTAREA" || target.isContentEditable) {
                return;
            }

            const isMod = e.ctrlKey || e.metaKey;

            if (isMod && e.key === "c") copy();
            if (isMod && e.key === "v") paste();
            if (isMod && e.key === "d") duplicate(e);
            if (isMod && e.key === "z") undo();
        };

        window.addEventListener("keydown", handleKeyDown);
        return () => window.removeEventListener("keydown", handleKeyDown);
    }, [enabled, copy, paste, duplicate, undo]);

    return { pushHistory };
}