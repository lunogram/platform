import { useState, useEffect, useCallback, useRef } from "react";
import api, { apiUrl } from "@/api";
import { toast } from "react-hot-toast/headless";
import { useTranslation } from "react-i18next";
import type { User } from "@/types";

const STORAGE_KEY = (projectId: string, journeyId: string) => 
  `journey_follow_${projectId}_${journeyId}`;

export function useUserSelection(projectId: string, journeyId: string, isOpen: boolean, onUserEnteredNode: (external_id: string) => void) {
  const { t } = useTranslation();
  const [users, setUsers] = useState<User[]>([]);
  const [isRestoring, setIsRestoring] = useState(false);

  useEffect(() => {
    if (isOpen && users.length === 0) {
      api.users.list(projectId, { limit: 100 }).then((r) => setUsers(r.results));
    }
  }, [isOpen, projectId, users.length]);
  const eventSourceRef = useRef<EventSource | null>(null);
  const activeUserIdRef = useRef<string | null>(null);
  const onUserEnteredNodeRef = useRef(onUserEnteredNode);
  onUserEnteredNodeRef.current = onUserEnteredNode;

  const stopFollowing = useCallback(() => {
    if (eventSourceRef.current) {
      eventSourceRef.current.close();
      eventSourceRef.current = null;
    }
    activeUserIdRef.current = null;
    sessionStorage.removeItem(STORAGE_KEY(projectId, journeyId));
  }, [projectId, journeyId]);

  const restoreFollowing = useCallback(async (userId: string) => {
    setIsRestoring(true);
    try {
      const states = await api.journeys.users.getState(projectId, journeyId, userId);
      
      for (const state of states) {
        onUserEnteredNodeRef.current(state.external_step_id);
      }

      followUser(userId);
    } catch (e) {
      console.error("Failed to restore user state:", e);
      followUser(userId);
    } finally {
      setIsRestoring(false);
    }
  }, [projectId, journeyId]);

  useEffect(() => {
    const stored = sessionStorage.getItem(STORAGE_KEY(projectId, journeyId));
    if (stored) {
      restoreFollowing(stored);
    }
  }, []);

  useEffect(() => {
    return () => {
      if (eventSourceRef.current) {
        eventSourceRef.current.close();
      }
    };
  }, []);

  const followUser = useCallback((userId: string) => {
    activeUserIdRef.current = userId;
    sessionStorage.setItem(STORAGE_KEY(projectId, journeyId), userId);

    if (eventSourceRef.current) {
      eventSourceRef.current.close();
    }

    const es = new EventSource(
      apiUrl(projectId, `journeys/${journeyId}/users/${userId}`),
      { withCredentials: true }
    );

    es.addEventListener("message", e => console.log("message:", e.data));
    es.addEventListener("step", e => {
      const data = JSON.parse(JSON.parse(e.data));
      onUserEnteredNodeRef.current(data.external_step_id);
      
      if (data.step_type === "exit") {
        stopFollowing();
      }
    });

    es.onerror = (e) => {
      console.error("EventSource error:", e);
      if (eventSourceRef.current) {
        eventSourceRef.current.close();
      }
    };

    eventSourceRef.current = es;
  }, [projectId, journeyId, stopFollowing]);

  const triggerUser = useCallback(async (stepId: string, userId: string) => {
    try {
      followUser(userId);
      await api.journeys.users.trigger(projectId, journeyId, stepId, userId);
      toast.success(t("user_triggered"));
    } catch (e) {
      toast.error(`Error: ${e}`);
    }
  }, [projectId, journeyId, t, followUser]);

  const skipDelay = useCallback(async (stepId: string, userId: string) => {
    try {
      await api.journeys.users.skipDelay(projectId, journeyId, userId, stepId);
      toast.success(t("user_skipped"));
    } catch (e) {
      toast.error(`Error: ${e}`);
    }
  }, [projectId, journeyId, t]);

  const skipDelayForActiveUser = useCallback(async (stepId: string) => {
    const userId = activeUserIdRef.current;
    if (!userId) {
      toast.error("No active user selected");
      return;
    }
    await skipDelay(stepId, userId);
  }, [skipDelay]);

  return { users, triggerUser, followUser, skipDelay, skipDelayForActiveUser, activeUserId: activeUserIdRef, isRestoring };
}