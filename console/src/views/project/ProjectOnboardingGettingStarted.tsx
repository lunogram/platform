import { Link, useNavigate, useParams } from "react-router";
import { useTranslation } from "react-i18next";
import { CampaignsIcon, JourneysIcon } from "../../components/icons";
import api from "../../api";
import type { UUID } from "@/types/common";
import { useState } from "react";
import { NIL } from "uuid";
import { Button } from "@/components/ui/button";

export default function ProjectOnboarding() {
  const navigate = useNavigate();
  const { t } = useTranslation();
  const { projectId = NIL as UUID } = useParams<{ projectId: UUID }>();
  const [isJourneyLoading, setIsJourneyLoading] = useState(false);

  async function createOnboardingJourney() {
    setIsJourneyLoading(true);
    try {
      // NOTE: check if any journey already exists
      const journeys = await api.journeys.search(projectId, { limit: 1 });
      if (journeys.results.length > 0) {
        await navigate(
          `/projects/${projectId}/journeys/${journeys.results[0].id}`,
        );
        return;
      }

      // NOTE: no journeys exist - create one
      const journey = await api.journeys.create(projectId, {
        name: "Onboarding",
        description: "Getting started with your first journey",
        template_id: "onboarding",
        status: "draft",
      });

      await navigate(`/projects/${projectId}/journeys/${journey.id}`);
    } finally {
      setIsJourneyLoading(false);
    }
  }

  async function createCampaign() {
    await navigate(`/projects/${projectId}/campaigns/new`);
  }

  return (
    <div className="getting-started-step">
      <h1 className="legacy-typography">{t("getting-started")}</h1>

      <section className="selection">
        <button onClick={createOnboardingJourney}>
          {!isJourneyLoading && (
            <>
              <CampaignsIcon />
              <span>{t("onboarding_project-getting-started_journey")}</span>
            </>
          )}
          {isJourneyLoading && <div className="is-loading"></div>}
        </button>
        <button onClick={createCampaign}>
          <JourneysIcon />
          <span>{t("onboarding_project-getting-started_campaign")}</span>
        </button>
      </section>

      <div className="flex gap-2 mt-4">
        <Link to={`/projects/${projectId}/getting-started`}>
          <Button variant="secondary">{t("skip")}</Button>
        </Link>
      </div>
    </div>
  );
}
