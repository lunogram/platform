import {
  useCallback,
  useContext,
  useEffect,
  useRef,
  useState,
} from "react";
import { useNavigate } from "react-router";
import type { ReactFlowInstance } from "reactflow";
import ReactFlow, {
  Background,
  Controls,
  MiniMap,
  Panel,
  useEdgesState,
  useNodesState,
} from "reactflow";
import { JourneyContext, ProjectContext } from "../../../contexts";
import { createComparator } from "../../../utils";
import * as journeySteps from "../steps/index";
import clsx from "clsx";
import api from "../../../api";
import { Button } from "@/components/ui/button";
import Modal from "../../../ui/Modal";
import { JourneyForm } from "../JourneyForm";
import Tag from "../../../ui/Tag";
import { useTranslation } from "react-i18next";
import { JourneyStepUsers } from "../JourneyStepUsers";
import type { UUID } from "@/types/common";
import { UserSelectionModal } from "../JourneyUserSelectionModal";

import { JourneyStepNode } from "../components/JourneyStepNode";
import { JourneyStepEdge } from "../components/JourneyStepEdge";
import {
  DATA_FORMAT,
} from "../hooks/JourneyEditor.constants";
import type { JourneyNodeData } from "../JourneyEditor.types";
import {
  cloneNodes,
  getStepType,
  stepsToNodes,
} from "./JourneyEditor.utils";

import "./JourneyEditor.css";
import "reactflow/dist/style.css";
import { useJourneyPersistence } from "../hooks/useJourneyPersistence";
import { useJourneyFlowHandlers } from "../hooks/useJourneyFlowHandlers";
import { useUserSelection } from "../hooks/useUserSelection";
import { useStepEditing } from "../hooks/useStepEditing";
import { JourneyStepSidebar } from "../components/JourneyStepSidebar";

const nodeTypes = { step: JourneyStepNode };
const edgeTypes = { step: JourneyStepEdge };

export default function JourneyEditor() {
  const navigate = useNavigate();
  const { t } = useTranslation();
  const wrapper = useRef<HTMLDivElement>(null);
  const [project] = useContext(ProjectContext);
  const [journey, setJourney] = useContext(JourneyContext);
  const [flowInstance, setFlowInstance] = useState<null | ReactFlowInstance>(
    null,
  );

  const [nodes, setNodes, onNodesChange] = useNodesState<JourneyNodeData>([]);
  const [edges, setEdges, onEdgesChange] = useEdgesState([]);
  const [viewUsersStep, setViewUsersStep] = useState<null | {
    stepId: UUID;
    stepType: string;
  }>(null);
  const [editOpen, setEditOpen] = useState(false);
  const [isUserModalOpen, setIsUserModalOpen] = useState(false);

  const {
    saving,
    publishing,
    hasUnsavedChanges,
    setHasUnsavedChanges,
    saveSteps,
    publishJourney,
    saveDraft,
  } = useJourneyPersistence(project, journey, setNodes, setEdges);

  const { editNode, selected, updateEditNode, deleteNode, updateNodes } =
    useStepEditing(nodes, setNodes, () => setHasUnsavedChanges(true));

  const { onConnect, onDrop, onNodeDoubleClick } = useJourneyFlowHandlers(
    nodes,
    setNodes,
    setEdges,
    flowInstance,
    wrapper,
    () => setHasUnsavedChanges(true),
  );

  const { users, triggerUser } = useUserSelection(
    project.id,
    journey.id,
    isUserModalOpen,
  );

  // Load initial data
  useEffect(() => {
    const load = async () => {
      const steps = await api.journeys.steps.get(project.id, journey.id);
      const { edges, nodes } = stepsToNodes(steps, { setViewUsersStep });
      setNodes(nodes);
      setEdges(edges);
    };
    void load();
  }, [project.id, journey.id, setNodes, setEdges]);

  const onPaneClick = useCallback(() => {
    if (editNode)
      setNodes(
        nodes.map((n) => ({ ...n, data: { ...n.data, editing: false } })),
      );
  }, [editNode, nodes, setNodes]);

  return (
    <Modal
      size="fullscreen"
      title={journey.name}
      open={true}
      onClose={() => navigate("../journeys")}
      actions={
        journey.status === "archived" ? (
          <Tag variant="error" size="large">
            {t("journey_archived")}
          </Tag>
        ) : journey.status === "draft" ? (
          <>
            <Button variant="secondary" onClick={() => setEditOpen(true)}>
              {t("edit_details")}
            </Button>

            <Button
              onClick={() => publishJourney(nodes, edges)}
              isLoading={publishing}
            >
              {t("publish")}
            </Button>
            <Button onClick={() => saveSteps(nodes, edges)} isLoading={saving}>
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
            {journey.draft_id ? (
              <Button
                onClick={() =>
                  (window.location.href = `/projects/${project.id}/journeys/${journey.draft_id}`)
                }
              >
                {t("journey_draft_edit")}
              </Button>
            ) : (
              <Button
                onClick={() => saveDraft(nodes, edges)}
                isLoading={saving}
              >
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
            onClick={onPaneClick}
            nodesDraggable={journey.status === "draft"}
            nodesConnectable={journey.status === "draft"}
            panOnScroll
            selectNodesOnDrag
            fitView
          >
            <Background className="internal-canvas" />
            {!editNode && (
              <>
                <Controls showInteractive={journey.status === "draft"} />
                <MiniMap
                  nodeClassName={(n) =>
                    `journey-minimap ${getStepType(n.data.type)?.category ?? "unknown"}`
                  }
                />
                {journey.status === "draft" && (
                  <Panel position="top-left">
                    {selected.length ? (
                      <Button
                        onClick={() => {
                          const { nodeCopies, edgeCopies } = cloneNodes(
                            edges,
                            nodes.filter((n) => n.selected),
                          );
                          updateNodes([
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

        {journey.status === "draft" && (
          <div className="journey-options">
            {editNode ? (
              <JourneyStepSidebar
                editNode={editNode}
                nodes={nodes}
                project={project}
                journey={journey}
                hasUnsavedChanges={hasUnsavedChanges}
                onUpdate={updateEditNode}
                onDelete={deleteNode}
                onOpenUserModal={() => setIsUserModalOpen(true)}
                onViewUsers={(stepId, stepType) => setViewUsersStep({ stepId, stepType })}
              />
            ) : (
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
          if (editNode?.id) triggerUser(editNode.id, u.id);
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
