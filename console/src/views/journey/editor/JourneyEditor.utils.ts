import type { Edge, Node } from "reactflow";
import { MarkerType, getConnectedEdges } from "reactflow";
import * as journeySteps from "../steps/index";
import { createUuid } from "@/utils";
import type { JourneyStepMap, JourneyStepType } from "@/types";
import { STEP_STYLE } from "../hooks/JourneyEditor.constants";
import type { JourneyNode, JourneyNodeData } from "./JourneyEditor.types";
import type { UUID } from "@/types/common";

export const getStepType = (type: string) =>
  (type ? (journeySteps[type as keyof typeof journeySteps] as JourneyStepType) : null) ?? null;

export const getSourcePath = (handleId: string) => handleId.substring(0, handleId.indexOf("-s-"));

interface CreateEdgeParams {
  sourceId: string;
  targetId: string;
  data: Record<string, unknown>;
  path?: string;
}

export function createEdge({ data, sourceId, targetId, path }: CreateEdgeParams): Edge {
  return {
    id: "e-" + sourceId + "__" + targetId,
    source: sourceId,
    sourceHandle: (path ?? "") + "-s-" + sourceId,
    target: targetId,
    targetHandle: "t-" + targetId,
    data,
    type: STEP_STYLE,
    markerEnd: {
      type: MarkerType.ArrowClosed,
    },
  };
}

export function stepsToNodes(
  stepMap: JourneyStepMap,
  actions: { setViewUsersStep?: (step: { stepId: UUID; stepType: string }) => void }
) {
  const nodes: JourneyNode[] = [];
  const edges: Edge[] = [];

  for (const [id, { x, y, type, data, name, data_key, children, stats, stats_at, id: stepId }] of Object.entries(stepMap)) {
    nodes.push({
      id,
      position: { x, y },
      type: "step",
      data: { type, name, data_key, data, stats, stats_at, stepId, ...actions },
    });
    children?.forEach(({ external_id, path, data }) =>
      edges.push(createEdge({ sourceId: id, targetId: external_id, data, path }))
    );
  }
  return { nodes, edges };
}

export function nodesToSteps(nodes: Node<JourneyNodeData>[], edges: Edge[]) {
  return nodes.reduce<JourneyStepMap>((a, { id, data: { type, name = "", data_key, data = {} }, position: { x, y } }) => {
    a[id] = {
      type,
      data,
      name,
      data_key,
      x,
      y,
      children: edges
        .filter((e) => e.source === id)
        .map(({ data = {}, sourceHandle, target }) => ({
          external_id: target,
          path: getSourcePath(sourceHandle!),
          data,
        })),
    };
    return a;
  }, {});
}

export function cloneNodes(edges: Edge[], targets: Node<JourneyNodeData>[]) {
  const mapping: { [prev: string]: string } = {};
  const nodeCopies: Node<JourneyNodeData>[] = [];
  for (const node of targets) {
    const id = createUuid();
    mapping[node.id] = id;
    nodeCopies.push({
      ...node,
      data: {
        ...(node.data ?? {}),
        name: node.data.name ? node.data.name + " (Copy)" : undefined,
      },
      id,
      position: { x: node.position.x + 50, y: node.position.y + 50 },
    });
  }
  const edgeCopies = getConnectedEdges(targets, edges)
    .filter((edge) => edge.source in mapping && edge.target in mapping)
    .map((edge) =>
      createEdge({
        sourceId: mapping[edge.source],
        targetId: mapping[edge.target],
        data: edge.data ?? {},
        path: getSourcePath(edge.sourceHandle!),
      })
    );
  return { nodeCopies, edgeCopies };
}