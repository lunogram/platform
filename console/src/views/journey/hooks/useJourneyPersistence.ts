import { useState, useCallback, type SetStateAction, type Dispatch } from "react";
import api from "@/api";
import { toast } from "react-hot-toast/headless";
import { useTranslation } from "react-i18next";
import { stepsToNodes, nodesToSteps } from "../editor/JourneyEditor.utils";
import type { JourneyNode } from "../editor/JourneyEditor.types";
import type { Edge } from "reactflow";
import type { Journey, Project } from "@/types";

export function useJourneyPersistence(project: Project, journey: Journey, setNodes: Dispatch<SetStateAction<JourneyNode[]>>, setEdges: Dispatch<SetStateAction<Edge[]>>) {
  const { t } = useTranslation();
  const [saving, setSaving] = useState(false);
  const [publishing, setPublishing] = useState(false);
  const [hasUnsavedChanges, setHasUnsavedChanges] = useState(false);

  const saveDraft = useCallback(async (nodes: JourneyNode[], edges: Edge[]) => {
    const stepMap = await api.journeys.steps.set(
      project.id,
      journey.id,
      nodesToSteps(nodes, edges)
    );
    return stepsToNodes(stepMap, {}); 
  }, [project.id, journey.id]);

  const saveSteps = useCallback(async (nodes: JourneyNode[], edges: Edge[]) => {
    setSaving(true);
    try {
      const refreshed = await saveDraft(nodes, edges);
      setNodes(refreshed.nodes);
      setEdges(refreshed.edges);
      setHasUnsavedChanges(false);
      toast.success(t("journey_saved"));
    } catch (e) {
      toast.error(`Error: ${e}`);
    } finally {
      setSaving(false);
    }
  }, [saveDraft, setNodes, setEdges, t]);

  const publishJourney = useCallback(async (nodes: JourneyNode[], edges: Edge[]) => {
    if (!confirm(t("journey_publish_confirmation"))) return;
    if (hasUnsavedChanges) await saveDraft(nodes, edges);
    setPublishing(true);
    try {
      await api.journeys.publish(project.id, journey.id);
      window.location.href = `/projects/${project.id}/journeys/${journey.parent_id ?? journey.id}`;
    } finally {
      setPublishing(false);
    }
  }, [project.id, journey, hasUnsavedChanges, saveDraft, t]);

  return { saving, publishing, hasUnsavedChanges, setHasUnsavedChanges, saveSteps, publishJourney, saveDraft };
}