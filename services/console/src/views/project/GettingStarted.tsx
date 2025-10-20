import { useContext, useEffect, useState } from 'react'
import { BookIcon, CampaignsIcon, CheckCircleIcon, JourneysIcon, ListsIcon, UsersIcon } from '../../ui/icons'
import Button, { LinkButton } from '../../ui/Button'
import { ProjectContext } from '../../contexts'
import { useNavigate, useParams } from 'react-router'
import type { UUID } from '@/types/common'
import { NIL } from 'uuid'
import api from '../../api'

import './GettingStarted.css'

export default function ProjectGettingStarted() {
    const navigate = useNavigate()
    const { projectId = NIL as UUID } = useParams<{ projectId: UUID }>()
    const [project, setProject] = useContext(ProjectContext)
    const [isJourneyLoading, setIsJourneyLoading] = useState(false)

    useEffect(() => {
        const loadProject = async () => {
            const projectState = await api.projects.get(projectId)
            setProject(projectState)
        }

        loadProject().catch((err) => console.error(err))
    }, [setProject, projectId])

    const hasCampaigns = (project.campaigns_count ?? 0) > 0
    const hasJourneys = (project.journeys_count ?? 0) > 0
    const hasUsers = (project.users_count ?? 0) > 0
    const hasLists = (project.lists_count ?? 0) > 0

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

    return (
        <div className="project-getting-started">
            <section className="getting-started-container getting-started-checklist">
                <header>
                    <h4>Onboarding checklist</h4>
                </header>
                <main>
                    <ul>
                        <li className="getting-started-checklist-item">
                            <div className={`icon ${hasCampaigns ? 'completed' : ''}`}>
                                {hasCampaigns ? <CheckCircleIcon /> : <CampaignsIcon />}
                            </div>
                            <div className="getting-started-checklist-item-content">
                                <strong>Create your first campaign</strong>
                                <small>Send a one-time message like a newsletter or announcement</small>
                            </div>
                            {!hasCampaigns && <LinkButton to="../campaigns" size="regular">Create Campaign</LinkButton>}
                        </li>
                        <li className="getting-started-checklist-item">
                            <div className={`icon ${hasJourneys ? 'completed' : ''}`}>
                                {hasJourneys ? <CheckCircleIcon /> : <JourneysIcon />}
                            </div>
                            <div className="getting-started-checklist-item-content">
                                <strong>Create your first Journey</strong>
                                <small>Automate messages based on user actions or scheduled events</small>
                            </div>
                            {!hasJourneys && <Button size="regular" onClick={createOnboardingJourney} isLoading={isJourneyLoading}>Create Journey</Button>}
                        </li>
                        <li className="getting-started-checklist-item">
                            <div className={`icon ${hasUsers ? 'completed' : ''}`}>
                                {hasUsers ? <CheckCircleIcon /> : <UsersIcon />}
                            </div>
                            <div className="getting-started-checklist-item-content">
                                <strong>Add your first users</strong>
                                <small>Upload a CSV or connect one of your data sources</small>
                            </div>
                            {!hasUsers && <LinkButton to="../users" size="regular">Onboard Users</LinkButton>}
                        </li>
                        <li className="getting-started-checklist-item">
                            <div className={`icon ${hasLists ? 'completed' : ''}`}>
                                {hasLists ? <CheckCircleIcon /> : <ListsIcon />}
                            </div>
                            <div className="getting-started-checklist-item-content">
                                <strong>Create your first list</strong>
                                <small>Segment your users into lists for targeted campaigns</small>
                            </div>
                            {!hasLists && <LinkButton to="../lists" size="regular">Create List</LinkButton>}
                        </li>
                    </ul>
                </main>
            </section>

            <section className="getting-started-container getting-started-resources">
                <div>
                    <div className="icon">
                        <BookIcon />
                    </div>
                    <h4>Documentation</h4>
                    <p>Explore our comprehensive guides and API documentation to get the most out of Lunogram.</p>
                </div>
            </section>
        </div>
    )
}
