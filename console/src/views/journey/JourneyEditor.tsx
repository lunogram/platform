import type { DragEventHandler, ReactNode } from "react";
import {
  useCallback,
  useContext,
  useEffect,
  useRef,
  useState,
  createElement,
} from "react";
import { useBlocker, useNavigate } from "react-router";
import type {
  ReactFlowInstance,
  Connection,
  NodeMouseHandler,
} from "reactflow";
import ReactFlow, {
  addEdge,
  Background,
  Controls,
  MiniMap,
  Panel,
  useEdgesState,
  useNodesState,
} from "reactflow";
import { JourneyContext, ProjectContext } from "../../contexts";
import { createComparator, createUuid } from "../../utils";
import * as journeySteps from "./steps/index";
import clsx from "clsx";
import api from "../../api";
import { Button } from "@/components/ui/button";
import Modal from "../../ui/Modal";
import { toast } from "react-hot-toast/headless";
import { JourneyForm } from "./JourneyForm";
import Tag from "../../ui/Tag";
import TextInput from "../../ui/form/TextInput";
import { useTranslation } from "react-i18next";
import { JourneyStepUsers } from "./JourneyStepUsers";
import { Menu, MenuItem } from "../../ui";
import type { UUID } from "@/types/common";
import type { User } from "../../types";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { UserSelectionModal } from "./JourneyUserSelectionModal";

import { JourneyStepNode } from "./editor/JourneyStepNode";
import { JourneyStepEdge } from "./editor/JourneyStepEdge";
import {
  DATA_FORMAT,
  STEP_STYLE,
  statIcons,
  stepCategoryColors,
} from "./editor/JourneyEditor.constants";
import type {
  JourneyNode,
  JourneyNodeData,
} from "./editor/JourneyEditor.types";
import {
  cloneNodes,
  getStepType,
  nodesToSteps,
  stepsToNodes,
} from "./editor/JourneyEditor.utils";

import "./JourneyEditor.css";
import "reactflow/dist/style.css";

const nodeTypes = { step: JourneyStepNode };
const edgeTypes = { step: JourneyStepEdge };

export default function JourneyEditor() {
  const navigate = useNavigate();
  const { t } = useTranslation();
  const [flowInstance, setFlowInstance] = useState<null | ReactFlowInstance>(
    null,
  );
  const wrapper = useRef<HTMLDivElement>(null);

  const [project] = useContext(ProjectContext);
  const [journey, setJourney] = useContext(JourneyContext);

  const [nodes, setNodes, onNodesChange] = useNodesState<JourneyNodeData>([]);
  const [edges, setEdges, onEdgesChange] = useEdgesState([]);

  const isDeleted = !!journey.deleted_at;
  const isDraft = journey.status === "draft" && !isDeleted;
  const draftId = journey.draft_id;
  const parentId = journey.parent_id;

  const [publishing, setPublishing] = useState(false);
  const [saving, setSaving] = useState(false);
  const [hasUnsavedChanges, setHasUnsavedChanges] = useState(false);
  const [viewUsersStep, setViewUsersStep] = useState<null | {
    stepId: UUID;
    stepType: string;
  }>(null);
  const [editOpen, setEditOpen] = useState(false);
  const [isUserModalOpen, setIsUserModalOpen] = useState(false);
  const [users, setUsers] = useState<User[]>([]);

  const handleSetNodes = useCallback(
    (nds: JourneyNode[]) => {
      setHasUnsavedChanges(true);
      setNodes(nds);
    },
    [setNodes],
  );

  const loadSteps = useCallback(async () => {
    const steps = await api.journeys.steps.get(project.id, journey.id);
    const { edges, nodes } = stepsToNodes(steps, { setViewUsersStep });
    setNodes(nodes);
    setEdges(edges);
  }, [project.id, journey.id, setNodes, setEdges]);

  useEffect(() => {
    void loadSteps();
  }, [loadSteps]);

  const blocker = useBlocker(
    ({ currentLocation, nextLocation }) =>
      hasUnsavedChanges && currentLocation.pathname !== nextLocation.pathname,
  );

  useEffect(() => {
    if (blocker.state === "blocked") {
      if (confirm(t("confirm_unsaved_changes"))) blocker.proceed();
      else blocker.reset();
    }
  }, [blocker, t]);

  const saveDraft = useCallback(async () => {
    const stepMap = await api.journeys.steps.set(
      project.id,
      journey.id,
      nodesToSteps(nodes, edges),
    );
    const refreshed = stepsToNodes(stepMap, { setViewUsersStep });
    setNodes(refreshed.nodes);
    setEdges(refreshed.edges);
  }, [project, journey, nodes, edges, setNodes, setEdges]);

  const saveSteps = useCallback(async () => {
    setSaving(true);
    try {
      await saveDraft();
      toast.success(t("journey_saved"));
    } catch (e) {
      toast.error(`Error: ${e}`);
    } finally {
      setHasUnsavedChanges(false);
      setSaving(false);
    }
  }, [saveDraft, t]);

  const publishJourney = async () => {
    if (!confirm(t("journey_publish_confirmation"))) return;
    if (hasUnsavedChanges) await saveDraft();
    setPublishing(true);
    try {
      await api.journeys.publish(project.id, journey.id);
      window.location.href = `/projects/${project.id}/journeys/${journey.parent_id ?? journey.id}`;
    } finally {
      setPublishing(false);
    }
  };

  const createDraft = async () => {
    setSaving(true);
    try {
      const newDraft = await api.journeys.version(project.id, journey.id);
      setJourney(newDraft);
      window.location.href = `/projects/${project.id}/journeys/${newDraft.id}`;
    } finally {
      setSaving(false);
    }
  };

  // 5. FLOW HANDLERS
  const onConnect = useCallback(
    async (conn: Connection) => {
      const sourceNode = nodes.find((n) => n.id === conn.source);
      const stepType = sourceNode?.data.type;
      const data = stepType
        ? ((await getStepType(stepType)?.newEdgeData?.()) ?? {})
        : {};
      setEdges((eds) => addEdge({ ...conn, type: STEP_STYLE, data }, eds));
    },
    [nodes, setEdges],
  );

  const onDrop = useCallback<DragEventHandler>(
    async (event) => {
      event.preventDefault();
      if (!wrapper.current || !flowInstance) return;
      const bounds = wrapper.current.getBoundingClientRect();
      const payload = JSON.parse(event.dataTransfer.getData(DATA_FORMAT));
      const type = getStepType(payload.type);
      if (!type) return;
      const { x, y } = flowInstance.project({
        x: event.clientX - bounds.left - (payload.x ?? 0),
        y: event.clientY - bounds.top - (payload.y ?? 0),
      });
      handleSetNodes(
        (await (async (nds: JourneyNode[]) =>
          nds.concat({
            id: createUuid(),
            position: { x, y },
            type: "step",
            data: {
              type: payload.type,
              data: type.newData ? await type.newData() : {},
            },
          }))(nodes)) as JourneyNode[],
      );
    },
    [flowInstance, handleSetNodes, nodes],
  );

  const onNodeDoubleClick = useCallback<NodeMouseHandler>(
    (_, n) => {
      setNodes((nds) =>
        nds.map((x) =>
          x.id === n.id
            ? { ...x, data: { ...x.data, editing: true } }
            : { ...x, data: { ...x.data, editing: false } },
        ),
      );
      setTimeout(
        () =>
          flowInstance?.setCenter(n.position.x + 60, n.position.y + 60, {
            zoom: 1,
          }),
        10,
      );
    },
    [flowInstance, setNodes],
  );

  const selected = nodes.filter((n) => n.selected);
  const editNode = nodes.find((n) => n.data.editing);

  useEffect(() => {
    if (users.length > 0 || !isUserModalOpen) return;
    api.users.list(project.id, { limit: 100 }).then((r) => setUsers(r.results));
  }, [isUserModalOpen, project.id, users.length]);

  let stepEdit: ReactNode = null;
  if (editNode) {
    const type = editNode.data.type ? getStepType(editNode.data.type) : null;
    if (type) {
      const stats = editNode.data.stats ?? {};
      stepEdit = (
        <>
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
              {t(type.name)}
            </h4>
            <div
              className="step-header-stats"
              onClick={
                editNode.data.stepId
                  ? () =>
                      setViewUsersStep({
                        stepId: editNode.data.stepId!,
                        stepType: editNode.data.type!,
                      })
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
            {editNode.data.type === "entrance" && (
              <TooltipProvider>
                <Tooltip delayDuration={300}>
                  <TooltipTrigger asChild>
                    <div
                      className="step-header-stats"
                      onClick={() =>
                        !hasUnsavedChanges && setIsUserModalOpen(true)
                      }
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
              <MenuItem
                onClick={() =>
                  handleSetNodes(
                    nodes.filter((item) => item.id !== editNode.id),
                  )
                }
              >
                {t("delete_step")}
              </MenuItem>
            </Menu>
          </div>
          <div className="journey-options-edit">
            <TextInput
              name="stepName"
              label={t("name")}
              value={editNode.data.name ?? ""}
              onChange={(name) =>
                handleSetNodes(
                  nodes.map((n) =>
                    n.id === editNode.id
                      ? { ...n, data: { ...n.data, name } }
                      : n,
                  ),
                )
              }
            />
            {type.hasDataKey && (
              <TextInput
                name="dataKey"
                label={t("data_key")}
                value={editNode.data.data_key}
                onChange={(data_key) =>
                  handleSetNodes(
                    nodes.map((n) =>
                      n.id === editNode.id
                        ? { ...n, data: { ...n.data, data_key } }
                        : n,
                    ),
                  )
                }
              />
            )}
            {type.Edit &&
              createElement(type.Edit, {
                value: editNode.data.data ?? {},
                onChange: (data: Record<string, unknown>) =>
                  handleSetNodes(
                    nodes.map((n) =>
                      n.id === editNode.id
                        ? { ...n, data: { ...n.data, data } }
                        : n,
                    ),
                  ),
                project,
                journey,
                stepId: editNode.data.stepId,
                nodes,
              })}
          </div>
        </>
      );
    }
  }

  return (
    <Modal
      size="fullscreen"
      title={journey.name}
      open={true}
      onClose={() => navigate("../journeys")}
      actions={
        isDeleted ? (
          <Tag variant="error" size="large">
            {t("journey_archived")}
          </Tag>
        ) : isDraft ? (
          <>
            <Button variant="secondary" onClick={() => setEditOpen(true)}>
              {t("edit_details")}
            </Button>
            <Button
              variant="secondary"
              onClick={publishJourney}
              isLoading={publishing}
            >
              {t("publish")}
            </Button>
            <Button onClick={saveSteps} isLoading={saving}>
              {t("journey_draft_save")}
            </Button>
          </>
        ) : (
          <>
            <Tag
              variant={journey.status === "published" ? "success" : "plain"}
              size="large"
            >
              {t(journey.status)}
            </Tag>
            <Button variant="secondary" onClick={() => setEditOpen(true)}>
              {t("edit_details")}
            </Button>
            {draftId ? (
              <Button
                onClick={() =>
                  (window.location.href = `/projects/${project.id}/journeys/${draftId}`)
                }
              >
                {t("journey_draft_edit")}
              </Button>
            ) : (
              <Button onClick={createDraft} isLoading={saving}>
                {t("journey_draft_create")}
              </Button>
            )}
          </>
        )
      }
    >
      <div className={clsx("journey", editNode && "editing")}>
        <div className="journey-builder" ref={wrapper}>
          <ReactFlow
            nodeTypes={nodeTypes}
            edgeTypes={edgeTypes}
            nodes={nodes}
            edges={edges}
            onNodesChange={onNodesChange}
            onEdgesChange={onEdgesChange}
            onConnect={onConnect}
            onInit={setFlowInstance}
            onNodeDoubleClick={onNodeDoubleClick}
            onDrop={onDrop}
            onDragOver={(e) => {
              e.preventDefault();
              e.dataTransfer.dropEffect = "move";
            }}
            onClick={() => {
              // THIS FIXES CLICKING AWAY TO RESET SIDEBAR
              if (editNode) {
                setNodes((nds) =>
                  nds.map((n) =>
                    n.data.editing
                      ? { ...n, data: { ...n.data, editing: false } }
                      : n,
                  ),
                );
              }
            }}
            nodesDraggable={isDraft}
            nodesConnectable={isDraft}
            panOnScroll
            selectNodesOnDrag
            fitView
          >
            <Background className="internal-canvas" />
            {!editNode && (
              <>
                <Controls showInteractive={isDraft} />
                <MiniMap
                  nodeClassName={(n) =>
                    `journey-minimap ${getStepType(n.data.type)?.category ?? "unknown"}`
                  }
                />
                {isDraft && (
                  <Panel position="top-left">
                    {selected.length ? (
                      <Button
                        onClick={() => {
                          const { nodeCopies, edgeCopies } = cloneNodes(
                            edges,
                            selected,
                          );
                          setNodes([
                            ...nodes.map((n) => ({ ...n, selected: false })),
                            ...nodeCopies,
                          ]);
                          setEdges([
                            ...edges.map((e) => ({ ...e, selected: false })),
                            ...edgeCopies,
                          ]);
                        }}
                        size="sm"
                      >
                        {`Duplicate Selected Steps (${selected.length})`}
                      </Button>
                    ) : (
                      "Shift+Drag to Multi Select"
                    )}
                  </Panel>
                )}
              </>
            )}
          </ReactFlow>
        </div>

        {isDraft && (
          <div className="journey-options">
            {stepEdit ?? (
              <>
                <h4 className="legacy-typography">{t("components")}</h4>
                {Object.entries(journeySteps)
                  .sort(createComparator((x) => x[1].category))
                  .map(([key, type]) => (
                    <div
                      key={key}
                      className={clsx("component", type.category)}
                      draggable
                      onDragStart={(event) => {
                        const rect = (
                          event.target as HTMLDivElement
                        ).getBoundingClientRect();
                        event.dataTransfer.setData(
                          DATA_FORMAT,
                          JSON.stringify({
                            type: key,
                            x: event.clientX - rect.left,
                            y: event.clientY - rect.top,
                          }),
                        );
                      }}
                    >
                      <span className={clsx("component-handle", type.category)}>
                        {type.icon}
                      </span>
                      <div className="component-title">{t(type.name)}</div>
                      <div className="component-desc">
                        {t(type.description)}
                      </div>
                    </div>
                  ))}
              </>
            )}
          </div>
        )}
      </div>

      <UserSelectionModal
        users={users}
        isOpen={isUserModalOpen}
        onClose={() => setIsUserModalOpen(false)}
        onSelect={(u) => {
          setIsUserModalOpen(false);
          if (!editNode || !editNode.id) return;

          api.journeys.users
            .trigger(project.id, journey.id, editNode.id, u.id)
            .then(() => {
              toast.success(t("user_triggered"));
            })
            .catch((e) => {
              toast.error(`Error: ${e}`);
            });

          console.log("Selected user for entrance step:", u);
          console.log("Step ID:", editNode);
        }}
      />

      <Modal
        open={editOpen}
        onClose={setEditOpen}
        title={t("edit_journey_details")}
      >
        <JourneyForm
          journey={journey}
          onSaved={async (j) => {
            setEditOpen(false);
            setJourney(j);
          }}
        />
      </Modal>

      {!!viewUsersStep && (
        <JourneyStepUsers
          open={!!viewUsersStep}
          onClose={() => setViewUsersStep(null)}
          stepType={viewUsersStep.stepType}
          stepId={viewUsersStep.stepId}
        />
      )}
    </Modal>
  );
}
