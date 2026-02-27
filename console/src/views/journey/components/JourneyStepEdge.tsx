import { memo, useCallback, useContext, useMemo, createElement } from "react";
import type { EdgeProps } from "reactflow";
import {
  getBezierPath,
  EdgeLabelRenderer,
  useNodes,
  useEdges,
  useReactFlow,
} from "reactflow";
import { ProjectContext, JourneyContext } from "@/contexts";
import { getStepType } from "../editor/JourneyEditor.utils";
import type { JourneyNode } from "../editor/JourneyEditor.types";

import "reactflow/dist/style.css";
import "../JourneyEditor.css";

export const JourneyStepEdge = memo(
  ({
    id,
    sourceX,
    sourceY,
    targetX,
    targetY,
    sourcePosition,
    targetPosition,
    source,
    sourceHandleId,
    targetHandleId,
    data = {},
  }: EdgeProps) => {
    const [project] = useContext(ProjectContext);
    const [journey] = useContext(JourneyContext);
    const nodes = useNodes() as JourneyNode[];
    const edges = useEdges();

    const siblingData = useMemo(
      () =>
        edges
          .filter(
            (e) =>
              e.sourceHandle === sourceHandleId &&
              e.targetHandle !== targetHandleId,
          )
          .map((e) => e.data ?? {}),
      [edges, sourceHandleId, targetHandleId],
    );

    const [edgePath, labelX, labelY] = getBezierPath({
      sourceX,
      sourceY,
      sourcePosition,
      targetX,
      targetY,
      targetPosition,
    });
    const { setEdges } = useReactFlow();

    const onChangeData = useCallback(
      (data: Record<string, unknown>) =>
        setEdges((edges) =>
          edges.map((e) => (e.id === id ? { ...e, data } : e)),
        ),
      [id, setEdges],
    );

    const sourceNode = nodes.find((n) => n.id === source);
    const sourceType = sourceNode?.data?.type
      ? getStepType(sourceNode.data.type)
      : null;

    return (
      <>
        <path id={id} className="react-flow__edge-path" d={edgePath} />
        {!!(sourceNode && sourceType?.EditEdge) && (
          <EdgeLabelRenderer>
            <div
              style={{
                position: "absolute",
                transform: `translate(-50%, -50%) translate(${labelX}px,${labelY}px)`,
              }}
              className="nodrag nopan journey-step-edge"
            >
              {createElement(sourceType.EditEdge, {
                value: data,
                onChange: onChangeData,
                stepData: sourceNode.data.data,
                siblingData,
                journey,
                project,
              })}
            </div>
          </EdgeLabelRenderer>
        )}
      </>
    );
  },
);
