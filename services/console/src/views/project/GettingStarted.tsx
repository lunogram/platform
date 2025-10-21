import { useContext, useEffect, useState } from 'react'

import {
    BookIcon,
    CampaignsIcon,
    CheckCircleIcon,
    JourneysIcon,
    ListsIcon,
    UsersIcon,
} from '@/components/icons'

import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { ProjectContext } from '@/contexts'
import { useNavigate, useParams } from 'react-router'
import type { UUID } from '@/types/common'
import { NIL } from 'uuid'
import api from '@/api'
import { cn } from '@/lib/utils'

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
        loadProject().catch(console.error)
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

    const checklistItems = [
        {
            icon: <CampaignsIcon />,
            completed: hasCampaigns,
            title: 'Create your first campaign',
            description: 'Send a one-time message like a newsletter or announcement',
            action: (
                <Button variant="secondary" onClick={() => navigate('../campaigns')}>
                    Create Campaign
                </Button>
            ),
        },
        {
            icon: <JourneysIcon />,
            completed: hasJourneys,
            title: 'Create your first Journey',
            description:
                'Automate messages based on user actions or scheduled events',
            action: (
                <Button
                    variant="secondary"
                    onClick={createOnboardingJourney}
                    isLoading={isJourneyLoading}
                >
                    Create Journey
                </Button>
            ),
        },
        {
            icon: <UsersIcon />,
            completed: hasUsers,
            title: 'Add your first users',
            description: 'Upload a CSV or connect one of your data sources',
            action: (
                <Button variant="secondary" onClick={() => navigate('../users')}>
                    Onboard Users
                </Button>
            ),
        },
        {
            icon: <ListsIcon />,
            completed: hasLists,
            title: 'Create your first list',
            description: 'Segment your users into lists for targeted campaigns',
            action: (
                <Button variant="secondary" onClick={() => navigate('../lists')}>
                    Create List
                </Button>
            ),
        },
    ]

    return (
        <div className="flex flex-col gap-6 max-w-3xl mx-auto min-h-screen justify-center">
            {/* Checklist */}
            <Card className="border rounded-lg">
                <CardHeader className="border-b">
                    <CardTitle className="text-lg font-semibold">Onboarding checklist</CardTitle>
                </CardHeader>
                <CardContent className="pt-2">
                    <ul className="divide-y divide-border">
                        {checklistItems.map((item, i) => (
                            <li
                                key={i}
                                className="flex items-center gap-4 py-4 first:pt-2 last:pb-2"
                            >
                                <div
                                    className={cn(
                                        'w-6 h-6 flex items-center justify-center shrink-0 text-muted-foreground',
                                        item.completed && 'text-green-600'
                                    )}
                                >
                                    {item.completed ? <CheckCircleIcon /> : item.icon}
                                </div>
                                <div className="flex flex-col flex-1 gap-1">
                                    <strong className="text-sm font-medium">{item.title}</strong>
                                    <small className="text-xs text-muted-foreground">
                                        {item.description}
                                    </small>
                                </div>
                                {!item.completed && item.action}
                            </li>
                        ))}
                    </ul>
                </CardContent>
            </Card>

            {/* Resources */}
            <Card className="border rounded-lg">
                <CardContent className="flex flex-col sm:flex-row gap-8 p-6">
                    <div className="flex-1 border-r border-border last:border-none pr-6">
                        <div className="w-6 h-6 mb-2 text-muted-foreground">
                            <BookIcon />
                        </div>
                        <h4 className="font-semibold mb-1">Documentation</h4>
                        <p className="text-sm text-muted-foreground">
                            Explore our comprehensive guides and API documentation to get the
                            most out of Lunogram.
                        </p>
                    </div>
                </CardContent>
            </Card>
        </div>
    )
}