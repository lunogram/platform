import { useState, useEffect, useCallback, useRef } from "react";
import api, { apiUrl } from "@/api";
import { toast } from "react-hot-toast/headless";
import { useTranslation } from "react-i18next";
import type { User } from "@/types";

export function useUserSelection(projectId: string, journeyId: string, isOpen: boolean) {
  const { t } = useTranslation();
  const [users, setUsers] = useState<User[]>([]);

  useEffect(() => {
    if (isOpen && users.length === 0) {
      api.users.list(projectId, { limit: 100 }).then((r) => setUsers(r.results));
    }
  }, [isOpen, projectId, users.length]);
  const eventSourceRef = useRef<EventSource | null>(null);

  const followUser = useCallback((userId: string) => {
    // Close existing connection if any
    if (eventSourceRef.current) {
      eventSourceRef.current.close();
    }

    const es = new EventSource(
      apiUrl(projectId, `journeys/${journeyId}/users?userID=${userId}`),
      { withCredentials: true }
    );

    es.addEventListener("message", e => console.log("message:", e.data));
    es.addEventListener("step", e => console.log("step:", e.data));

    es.onopen = () => console.log("SSE connected");
    es.onerror = e => console.log("SSE error", e);

    eventSourceRef.current = es;
  }, [projectId, journeyId]);

  const triggerUser = useCallback(async (stepId: string, userId: string) => {
    try {
      followUser(userId);
      await api.journeys.users.trigger(projectId, journeyId, stepId, userId);
      toast.success(t("user_triggered"));
    } catch (e) {
      toast.error(`Error: ${e}`);
    }
  }, [projectId, journeyId, t, followUser]);

  return { users, triggerUser, followUser };
}