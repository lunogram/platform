import { useNavigate, useParams } from 'react-router'
import { useTranslation } from 'react-i18next'
import { LinkButton } from '../../ui/Button'
import { CampaignsIcon, JourneysIcon } from '../../components/icons'
import api from '../../api'
import type { UUID } from '@/types/common'
import { useState } from 'react'
import { NIL } from 'uuid'

export default function ProjectOnboarding() {
    const navigate = useNavigate()
    const { t } = useTranslation()
    const { projectId = NIL as UUID } = useParams<{ projectId: UUID }>()
    const [isJourneyLoading, setIsJourneyLoading] = useState(false)

    async function createOnboardingJourney() {
        setIsJourneyLoading(true)
        try {
            const journey = await api.journeys.create(projectId, {
                name: 'Onboarding',
                description: 'Getting started with your first journey',
                template_id: 'onboarding',
                status: 'draft',
            })

            await navigate(`/projects/${projectId}/journeys/${journey.id}`)
        } finally {
            setIsJourneyLoading(false)
        }
    }

    async function createCampaign() {
        // TODO: create onboarding campaign
        await navigate(`/projects/${projectId}/campaigns`)
    }

    return (
        <div className="getting-started-step">
            <h1>{t('getting-started')}</h1>

            <section className="selection">
                <div onClick={createOnboardingJourney}>
                    {!isJourneyLoading && (<>
                        <CampaignsIcon />
                        <span>
                            {t('onboarding_project-getting-started_journey')}
                        </span>
                    </>)}
                    {isJourneyLoading && <div className="is-loading"></div>}
                </div>
                <div onClick={createCampaign}>
                    <JourneysIcon />
                    <span>
                        {t('onboarding_project-getting-started_campaign')}
                    </span>
                </div>
            </section>

            <div className="flex gap-2 mt-4">
                <LinkButton to={`/projects/${projectId}/getting-started`} variant="secondary">{t('skip')}</LinkButton>
            </div>
        </div>
    )
}
