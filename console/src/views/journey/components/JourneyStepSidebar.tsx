import { createElement } from "react";
import clsx from "clsx";
import { useTranslation } from "react-i18next";
import { Menu, MenuItem } from "@/ui";
import TextInput from "@/ui/form/TextInput";
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/components/ui/tooltip";
import { getStepType } from "../editor/JourneyEditor.utils";
import { stepCategoryColors, statIcons } from "../hooks/JourneyEditor.constants";
import type { JourneyNode } from "../editor/JourneyEditor.types";
import type { Project, Journey } from "@/types";

interface JourneyStepSidebarProps {
  editNode: JourneyNode;
  nodes: JourneyNode[];
  project: Project;
  journey: Journey;
  hasUnsavedChanges: boolean;
  onUpdate: (partial: Partial<JourneyNode["data"]>) => void;
  onDelete: (id: string) => void;
  onOpenUserModal: () => void;
  onViewUsers: (stepId: string, stepType: string) => void;
}

export function JourneyStepSidebar({
  editNode,
  nodes,
  project,
  journey,
  hasUnsavedChanges,
  onUpdate,
  onDelete,
  onOpenUserModal,
  onViewUsers,
}: JourneyStepSidebarProps) {
  const { t } = useTranslation();
  const type = editNode.data.type ? getStepType(editNode.data.type) : null;

  if (!type) return null;

  const stats = editNode.data.stats ?? {};

  return (
    <>
      <div className="journey-step-header">
        <span
          className={clsx(
            "step-header-icon",
            stepCategoryColors[type.category as keyof typeof stepCategoryColors]
          )}
        >
          {type.icon}
        </span>
        <h4 className="legacy-typography step-header-title">{t(type.name)}</h4>

        {/* Stats Section */}
        <div
          className="step-header-stats"
          onClick={
            editNode.data.stepId
              ? () => onViewUsers(editNode.data.stepId!, editNode.data.type!)
              : undefined
          }
        >
          <span className="stat">
            {stats.completed ?? 0} {statIcons.completed}
          </span>
          {(editNode.data.type === "delay" || !!stats.delay) && (
            <span className="stat">
              {stats.delay ?? 0} {statIcons.delay}
            </span>
          )}
          {(editNode.data.type === "action" || !!stats.action) && (
            <span className="stat">
              {stats.action ?? 0} {statIcons.action}
            </span>
          )}
        </div>

        {/* Entrance Step "Run" Logic */}
        {editNode.data.type === "entrance" && (
          <TooltipProvider>
            <Tooltip delayDuration={300}>
              <TooltipTrigger asChild>
                <div
                  className="step-header-stats"
                  role="button"
                  onClick={() => !hasUnsavedChanges && onOpenUserModal()}
                  style={{
                    cursor: hasUnsavedChanges ? "not-allowed" : "pointer",
                  }}
                >
                  <span className="stat">Run</span>
                </div>
              </TooltipTrigger>
              {hasUnsavedChanges && (
                <TooltipContent side="top">
                  <p>Save changes before running.</p>
                </TooltipContent>
              )}
            </Tooltip>
          </TooltipProvider>
        )}

        <Menu size="min">
          <MenuItem onClick={() => onDelete(editNode.id)}>{t("delete_step")}</MenuItem>
        </Menu>
      </div>

      <div className="journey-options-edit">
        <TextInput
          name="stepName"
          label={t("name")}
          value={editNode.data.name ?? ""}
          onChange={(name) => onUpdate({ name })}
        />
        
        {type.hasDataKey && (
          <TextInput
            name="dataKey"
            label={t("data_key")}
            value={editNode.data.data_key}
            onChange={(data_key) => onUpdate({ data_key })}
          />
        )}

        {/* Dynamic Type-Specific Editor */}
        {type.Edit &&
          createElement(type.Edit, {
            value: editNode.data.data ?? {},
            onChange: (data: Record<string, unknown>) => onUpdate({ data }),
            project,
            journey,
            stepId: editNode.data.stepId,
            nodes,
          })}
      </div>
    </>
  );
}