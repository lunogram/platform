import { useState, useEffect, useCallback } from "react";
import api from "@/api";
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

  const triggerUser = useCallback(async (stepId: string, userId: string) => {
    try {
      await api.journeys.users.trigger(projectId, journeyId, stepId, userId);
      toast.success(t("user_triggered"));
    } catch (e) {
      toast.error(`Error: ${e}`);
    }
  }, [projectId, journeyId, t]);

  return { users, triggerUser };
}