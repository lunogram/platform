import type { RouteObject } from 'react-router';
import { createBrowserRouter, Outlet, redirect, useNavigate, useParams } from 'react-router'
import api from '../api'

import ErrorPage from './ErrorPage'
import type { SidebarLink } from '../ui/Sidebar'
import { LoaderContextProvider, StatefulLoaderContextProvider } from './LoaderContextProvider'
import { AdminContext, CampaignContext, JourneyContext, ListContext, ProjectContext, UserContext } from '../contexts'
import ApiKeys from './settings/ApiKeys'
import EmailEditor from './campaign/editor/EmailEditor'
import Lists from './users/Lists'
import ListDetail from './users/ListDetail'
import Users from './users/Users'
import Teams from './settings/Teams'
import Subscriptions from './settings/Subscriptions'
import UserDetail from './users/UserDetail'
import { createStatefulRoute } from './createStatefulRoute'
import UserDetailAttrs from './users/UserDetailAttrs'
import UserDetailEvents from './users/UserDetailEvents'
import UserDetailSubscriptions from './users/UserDetailSubscriptions'
import CampaignDetail from './campaign/CampaignDetail'
import Campaigns from './campaign/Campaigns'
import CampaignDelivery from './campaign/CampaignDelivery'
import CampaignPreview from './campaign/CampaignPreview'
import CampaignOverview from './campaign/CampaignOverview'
import CampaignDesign from './campaign/CampaignDesign'
import Journeys from './journey/Journeys'
import JourneyEditor from './journey/JourneyEditor'
import ProjectSettings from './settings/ProjectSettings'
import Integrations from './settings/Integrations'
import Tags from './settings/Tags'
import Login from './auth/Login'
import LoginCallback from './auth/LoginCallback'
import Onboarding from './auth/Onboarding'
import OnboardingProject from './auth/OnboardingProject'
import { CampaignsIcon, CheckCircleIcon, JourneysIcon, ListsIcon, SettingsIcon, UsersIcon } from '@/components/icons'
import { Projects } from './project/Projects'
import { completedGettingStarted, getRecentProjects, pushRecentProject } from '../utils'
import Settings from './settings/Settings'
import ProjectSidebar from './project/ProjectSidebar'
import ProjectGettingStarted from './project/GettingStarted'
import ProjectOnboarding from './project/ProjectOnboarding'
import ProjectOnboardingGettingStarted from './project/ProjectOnboardingGettingStarted'
import ProjectOnboardingUsers from './project/ProjectOnboardingUsers'
import ProjectOnboardingTools from './project/ProjectOnboardingTools'
import Admins from './organization/Admins'
import OrganizationSettings from './organization/Settings'
import Locales from './settings/Locales'
import JourneyUserEntrances from './journey/JourneyUserEntrances'
import UserDetailJourneys from './users/UserDetailJourneys'
import EntranceDetails from './journey/EntranceDetails'
import { Translation } from 'react-i18next'
import Organization from './organization/Organization'
import DataSchema from './settings/DataSchema'
import type { UUID } from '@/types/common'
import type { Project } from '../types'

export const useRoute = (includeProject = true) => {
    const { projectId = '' } = useParams()
    const navigate = useNavigate()
    const parts: string[] = []
    if (includeProject) {
        parts.push('projects', projectId)
    }
    return (path: string) => {
        const newParts = [...parts, path]
        navigate('/' + newParts.join('/'))?.catch(e => {
            console.error('Failed to navigate to:', e)
        })
    }
}

export interface RouterProps {
    routes?: (routes: RouteObject[]) => RouteObject[]
    projectSidebarLinks?: <T extends SidebarLink>(links: T[]) => T[]
    orgSidebarLinks?: <T extends SidebarLink>(links: T[]) => T[]
}

export const createRouter = ({
    routes = routes => routes,
    projectSidebarLinks = links => links,
    orgSidebarLinks = links => links,
}: RouterProps) => createBrowserRouter(routes([
    {
        path: '/login',
        element: <Login />,
    },
    {
        path: '/login/:driver/callback',
        element: <LoginCallback />,
    },
    {
        path: '*',
        errorElement: <ErrorPage />,
        loader: async () => await api.profile.get(),
        element: (
            <LoaderContextProvider context={AdminContext}>
                <Outlet />
            </LoaderContextProvider>
        ),
        children: [
            {
                index: true,
                loader: async () => {
                    const recents = getRecentProjects()
                    if (recents.length) {
                        return redirect(`projects/${recents[0].id}`)
                    }
                    return redirect('organization')
                },
                element: <Projects />,
            },
            {
                path: 'onboarding',
                element: <Onboarding />,
                children: [
                    {
                        path: 'project',
                        element: <OnboardingProject />,
                    },
                ],
            },
            {
                path: 'organization',
                loader: async () => await api.organizations.get(),
                element: <Organization filter={orgSidebarLinks} />,
                children: [
                    {
                        index: true,
                        loader: async () => {
                            return redirect('projects')
                        },
                    },
                    {
                        path: 'projects',
                        element: <Projects />,
                    },
                    {
                        path: 'admins',
                        element: <Admins />,
                    },
                    {
                        path: 'settings',
                        element: <OrganizationSettings />,
                    },
                ],
            },
            {
                path: 'projects/:projectId',
                loader: async ({ params: { projectId = '' } }) => {
                    const project = await api.projects.get(projectId)
                    return project
                },
                element: (
                    <StatefulLoaderContextProvider context={ProjectContext}>
                        <Outlet />
                    </StatefulLoaderContextProvider>
                ),
                children: [
                    {
                        path: 'onboarding',
                        element: <ProjectOnboarding />,
                        children: [
                            {
                                index: true,
                                path: '',
                                element: <ProjectOnboardingTools />,
                            },
                            {
                                path: 'users',
                                element: <ProjectOnboardingUsers />,
                            },
                            {
                                path: 'getting-started',
                                element: <ProjectOnboardingGettingStarted />,
                            },
                        ],
                    },
                    {
                        path: '',
                        loader: async ({ params: { projectId = '' } }) => {
                            const project = await api.projects.get(projectId)
                            if (!sessionStorage.getItem('skippedOnboarding') && project.campaigns_count === 0 && project.journeys_count === 0 && project.users_count === 0 && project.lists_count === 0) {
                                return redirect('onboarding')
                            }
                        },
                        element: (
                            <ProjectSidebar
                                links={projectSidebarLinks([
                                    {
                                        key: 'getting-started',
                                        to: 'getting-started',
                                        children: <Translation>{t => t('getting-started')}</Translation>,
                                        icon: <CheckCircleIcon />,
                                        active: (project: Project) => {
                                            return !completedGettingStarted(project)
                                        },
                                        minRole: 'editor',
                                    },
                                    {
                                        key: 'campaigns',
                                        to: 'campaigns',
                                        children: <Translation>{t => t('campaigns')}</Translation>,
                                        icon: <CampaignsIcon />,
                                        minRole: 'editor',
                                    },
                                    {
                                        key: 'journeys',
                                        to: 'journeys',
                                        children: <Translation>{t => t('journeys')}</Translation>,
                                        icon: <JourneysIcon />,
                                        minRole: 'editor',
                                    },
                                    {
                                        key: 'users',
                                        to: 'users',
                                        children: <Translation>{t => t('users')}</Translation>,
                                        icon: <UsersIcon />,
                                    },
                                    {
                                        key: 'lists',
                                        to: 'lists',
                                        children: <Translation>{t => t('lists')}</Translation>,
                                        icon: <ListsIcon />,
                                        minRole: 'editor',
                                    },
                                    {
                                        key: 'settings',
                                        to: 'settings',
                                        children: <Translation>{t => t('settings')}</Translation>,
                                        icon: <SettingsIcon />,
                                        minRole: 'admin',
                                    },
                                ])}
                            >
                                <Outlet />
                            </ProjectSidebar>
                        ),
                        children: [
                            {
                                index: true,
                                loader: async ({ params: { projectId = '' } }) => {
                                    const project = await api.projects.get(projectId)
                                    if (project.role === 'support') {
                                        return redirect(`/projects/${project.id}/users`)
                                    }
                                    return redirect('campaigns')
                                },
                            },
                            {
                                path: 'getting-started',
                                element: <ProjectGettingStarted />,
                            },
                            createStatefulRoute({
                                path: 'campaigns',
                                apiPath: api.campaigns,
                                element: <Campaigns />,
                            }),
                            createStatefulRoute({
                                path: 'campaigns/:entityId',
                                apiPath: api.campaigns,
                                context: CampaignContext,
                                element: <CampaignDetail />,
                                children: [
                                    {
                                        index: true,
                                        element: <CampaignOverview />,
                                    },
                                    {
                                        path: 'design',
                                        element: <CampaignDesign />,
                                    },
                                    {
                                        path: 'delivery',
                                        element: <CampaignDelivery />,
                                    },
                                    {
                                        path: 'preview',
                                        element: <CampaignPreview />,
                                    },
                                ],
                            }),
                            createStatefulRoute({
                                path: 'campaigns/:entityId/editor',
                                apiPath: api.campaigns,
                                context: CampaignContext,
                                element: <EmailEditor />,
                            }),
                            createStatefulRoute({
                                path: 'journeys',
                                apiPath: api.journeys,
                                element: <Journeys />,
                            }),
                            createStatefulRoute({
                                path: 'users',
                                apiPath: api.users,
                                element: <Users />,
                            }),
                            createStatefulRoute({
                                path: 'users/:entityId',
                                apiPath: api.users,
                                context: UserContext,
                                element: <UserDetail />,
                                children: [
                                    {
                                        index: true,
                                        element: <UserDetailAttrs />,
                                    },
                                    {
                                        path: 'events',
                                        element: <UserDetailEvents />,
                                    },
                                    {
                                        path: 'subscriptions',
                                        element: <UserDetailSubscriptions />,
                                    },
                                    {
                                        path: 'journeys',
                                        element: <UserDetailJourneys />,
                                    },
                                ],
                            }),
                            createStatefulRoute({
                                path: 'lists',
                                apiPath: api.lists,
                                element: <Lists />,
                            }),
                            createStatefulRoute({
                                path: 'lists/:entityId',
                                apiPath: api.lists,
                                context: ListContext,
                                element: <ListDetail />,
                            }),
                            {
                                path: 'entrances/:entranceId',
                                loader: async ({ params }) => await api.journeys.entrances.log(params.projectId! as UUID, params.entranceId! as UUID),
                                element: <EntranceDetails />,
                            },
                            {
                                path: 'settings',
                                element: <Settings />,
                                children: [
                                    {
                                        index: true,
                                        element: <ProjectSettings />,
                                    },
                                    {
                                        path: 'team',
                                        element: <Teams />,
                                    },
                                    {
                                        path: 'locales',
                                        element: <Locales />,
                                    },
                                    {
                                        path: 'data',
                                        element: <DataSchema />,
                                    },
                                    {
                                        path: 'api-keys',
                                        element: <ApiKeys />,
                                    },
                                    {
                                        path: 'integrations',
                                        element: <Integrations />,
                                    },
                                    {
                                        path: 'subscriptions',
                                        element: <Subscriptions />,
                                    },
                                    {
                                        path: 'tags',
                                        element: <Tags />,
                                    },
                                ],
                            },
                        ],
                    },
                    createStatefulRoute({
                        path: 'journeys/:entityId',
                        apiPath: api.journeys,
                        context: JourneyContext,
                        element: <JourneyEditor />,
                        children: [
                            {
                                index: true,
                                element: <JourneyEditor />,
                            },
                            {
                                path: 'entrances',
                                element: <JourneyUserEntrances />,
                            },
                        ],
                    }),
                ],
            },
            {
                path: '*',
                element: <ErrorPage status={404} />,
                errorElement: <ErrorPage />,
            },
        ],
    },
]))
