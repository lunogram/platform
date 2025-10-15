import { useNavigate, useParams } from 'react-router'
import { useTranslation } from 'react-i18next'
import Button from '../../ui/Button'
import api from '../../api'
import { UUID } from 'crypto'
import { useState } from 'react'

export default function ProjectOnboarding() {
    const navigate = useNavigate()
    const { t } = useTranslation()
    const { projectId } = useParams<{ projectId: string }>()
    const [isLoading, setIsLoading] = useState(false)

    async function createOnboardingJourney() {
        setIsLoading(true)
        try {
            const journey = await api.journeys.create(projectId as UUID, {
                name: 'Onboarding',
                description: 'Getting started with your first journey',
                template_id: 'onboarding',
                status: 'draft',
            })

            await navigate(`/projects/${projectId}/journeys/${journey.id}`)
        } finally {
            setIsLoading(false)
        }
    }

    return (
        <div>
            <h1>{t('onboarding_journey_title')}</h1>
            <p>{t('onboarding_journey_description')}</p>

            <Button onClick={createOnboardingJourney} isLoading={isLoading}>{t('onboarding_journey_action')}</Button>
        </div>
    )
}
