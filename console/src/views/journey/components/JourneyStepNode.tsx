import {
  memo,
  useCallback,
  useContext,
  Fragment,
  createElement,
  useEffect,
} from "react";
import type { Connection, NodeProps } from "reactflow";
import { Handle, Position, useReactFlow, getConnectedEdges } from "reactflow";
import { useTranslation } from "react-i18next";
import clsx from "clsx";
import { FastForward, User } from "lucide-react";
import { ProjectContext, JourneyContext } from "@/contexts";
import Alert from "@/ui/Alert";
import { KeyIcon } from "@/components/icons";
import { getStepType } from "../editor/JourneyEditor.utils";
import { stepCategoryColors } from "../hooks/JourneyEditor.constants";

import "reactflow/dist/style.css";
import "../editor/JourneyEditor.css";

export const JourneyStepNode = memo(
  ({
    id,
    data: {
      stepId,
      type: typeName,
      name,
      data,
      data_key,
      editing,
      skipDelay,
      visited = false,
      active = false,
    } = {},
    selected,
  }: NodeProps & { active?: boolean }) => {
    const { t } = useTranslation();
    const [project] = useContext(ProjectContext);
    const [journey] = useContext(JourneyContext);

    const { getNode, getEdges, setNodes } = useReactFlow();

    const type = getStepType(typeName);
    const isExit = typeName === "exit" || name?.toLowerCase() === "exit";
    const isActiveVisual = active && !isExit;
    const isExitCompletedVisual = isExit && active;
    const isVisitedVisual = visited && !isExit;

    useEffect(() => {
      if (active && isExit) {
        const timer = setTimeout(() => {
          setNodes((nds) =>
            nds.map((node) => {
              return {
                ...node,
                data: { ...node.data, active: false, visited: false },
              };
            }),
          );
        }, 2000);
        return () => clearTimeout(timer);
      }
    }, [active, isExit, setNodes]);

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
    const categoryColorClass =
      stepCategoryColors[type.category as keyof typeof stepCategoryColors];
    const isValid = isExit ? true : type.validate ? type.validate(data) : true;

    return (
      <>
        {isActiveVisual && (
          <div
            className={clsx(
              "absolute -top-4 -right-4 z-50 flex h-8 w-8 items-center justify-center rounded-full text-white shadow-xl animate-in zoom-in-75 fade-in duration-200 border-2 border-white",
              categoryColorClass,
            )}
            style={{ pointerEvents: "none" }}
          >
            <User size={16} fill="currentColor" />
          </div>
        )}

        {!type.hideTopHandle && (
          <Handle type="target" position={Position.Top} id={"t-" + id} />
        )}

        <div
          className={clsx(
            "journey-step transition-all duration-300",
            type.category,
            selected && "selected",

            !isValid
              ? "error border-red-500 ring-2 ring-red-200"
              : isActiveVisual
                ? "border-[3px] border-orange-500 shadow-lg scale-105"
                : isExitCompletedVisual
                  ? "border-[3px] border-green-500 shadow-lg"
                  : isVisitedVisual
                    ? "border-[3px] border-green-500"
                    : "border border-gray-200",
            editing && "editing",
          )}
        >
          <div className="journey-step-header">
            <span className={clsx("step-header-icon", categoryColorClass)}>
              {type.icon}
            </span>
            <h4 className="legacy-typography step-header-title">
              {name || t(type.name)}
            </h4>
            {type.category !== "info" && (
              <div className="step-header-stats">
                {typeName === "delay" && (
                  <button
                    onClick={(e) => {
                      e.stopPropagation();
                      const targetStepId = stepId ?? id;
                      if (targetStepId) skipDelay?.(targetStepId);
                    }}
                    className="skip-delay-btn"
                  >
                    <FastForward size={14} className="fill-current" />
                    <span>{t("Skip")}</span>
                  </button>
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
