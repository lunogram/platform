import { memo, useCallback, useContext, Fragment, createElement } from "react";
import type { Connection, NodeProps } from "reactflow";
import { Handle, Position, useReactFlow, getConnectedEdges } from "reactflow";
import { useTranslation } from "react-i18next";
import clsx from "clsx";
import { ProjectContext, JourneyContext } from "@/contexts";
import Alert from "@/ui/Alert";
import { KeyIcon } from "@/components/icons";
import { getStepType } from "../editor/JourneyEditor.utils";
import { statIcons, stepCategoryColors } from "../hooks/JourneyEditor.constants";

import "reactflow/dist/style.css";
import "../JourneyEditor.css";

export const JourneyStepNode = memo(
  ({
    id,
    data: {
      stepId,
      type: typeName,
      name,
      data,
      data_key,
      stats = {},
      editing,
      setViewUsersStep,
    } = {},
    selected,
  }: NodeProps) => {
    const { t } = useTranslation();
    const [project] = useContext(ProjectContext);
    const [journey] = useContext(JourneyContext);
    const { getNode, getEdges } = useReactFlow();

    const type = getStepType(typeName);

    const validateConnection = useCallback(
      (conn: Connection) => {
        if (!type) return false;
        if (type.multiChildSources) return true;
        const sourceNode = conn.source && getNode(conn.source);
        if (!sourceNode) return true;
        const existing = getConnectedEdges([sourceNode], getEdges());
        return (
          existing.filter((e) => e.sourceHandle === conn.sourceHandle).length <
          1
        );
      },
      [type, getNode, getEdges],
    );

    if (!type) return <Alert variant="error" title="Invalid Step Type" />;

    const isValid = type.validate ? type.validate(data) : true;

    return (
      <>
        {!type.hideTopHandle && (
          <Handle type="target" position={Position.Top} id={"t-" + id} />
        )}
        <div
          className={clsx(
            "journey-step",
            type.category,
            selected && "selected",
            Array.isArray(type.sources) && "journey-step-labelled-sources",
            isValid ? "" : "error",
            editing && "editing",
          )}
        >
          <div className="journey-step-header">
            <span
              className={clsx(
                "step-header-icon",
                stepCategoryColors[
                  type.category as keyof typeof stepCategoryColors
                ],
              )}
            >
              {type.icon}
            </span>
            <h4 className="legacy-typography step-header-title">
              {name || t(type.name)}
            </h4>
            {type.category !== "info" && (
              <div
                className="step-header-stats"
                onClickCapture={
                  stepId
                    ? () =>
                        setViewUsersStep?.({
                          stepId: stepId,
                          stepType: typeName,
                        })
                    : undefined
                }
              >
                <span className="stat">
                  {(stats.completed ?? 0).toLocaleString()}{" "}
                  {statIcons.completed}
                </span>
                {(typeName === "delay" || !!stats.delay) && (
                  <span className="stat">
                    {(stats.delay ?? 0).toLocaleString()} {statIcons.delay}
                  </span>
                )}
                {(typeName === "action" || !!stats.action) && (
                  <span className="stat">
                    {(stats.action ?? 0).toLocaleString()} {statIcons.action}
                  </span>
                )}
              </div>
            )}
          </div>
          <div className="journey-step-body">
            {type.Describe &&
              createElement(type.Describe, {
                project,
                journey,
                value: data,
                onChange: () => {},
              })}
            {!!data_key && (
              <div
                className="data-key"
                style={{ marginTop: type.Describe ? 10 : undefined }}
              >
                <KeyIcon />
                {data_key}
              </div>
            )}
          </div>
        </div>
        {!type.hideBottomHandle &&
          (Array.isArray(type.sources) ? type.sources : [""]).map(
            (key, index, arr) => {
              const left = ((index + 1) / (arr.length + 1)) * 100 + "%";
              return (
                <Fragment key={key}>
                  {key && (
                    <span className="step-handle-label" style={{ left }}>
                      {key}
                    </span>
                  )}
                  <Handle
                    type="source"
                    position={Position.Bottom}
                    id={key + "-s-" + id}
                    isValidConnection={validateConnection}
                    style={{ left }}
                  />
                </Fragment>
              );
            },
          )}
      </>
    );
  },
);
