import type { RouteObject } from "react-router"
import { createBrowserRouter, Navigate, Outlet, redirect } from "react-router"
import api from "../api"
import oapiClient from "../oapi/client"

import ErrorPage from "./ErrorPage"
import { LoaderContextProvider, StatefulLoaderContextProvider } from "./LoaderContextProvider"
import {
    AdminContext,
    CampaignContext,
    TemplateContext,
    JourneyContext,
    ListContext,
    ProjectContext,
    UserContext,
    OrganizationContext,
} from "../contexts"
import ClientList from "./settings/clients/ClientList"
import { NewClientRoute, EditClientRoute } from "./settings/clients/ClientEditorLayout"
import Members from "./settings/Members"
import Lists from "./users/Lists"
import ListDetail from "./users/ListDetail"
import Users from "./users/Users"
import Subscriptions from "./settings/Subscriptions"
import EventSchemas from "./settings/EventSchemas"
import PushProviders from "./settings/PushProviders"
import UserDetail from "./users/UserDetail"
import { createStatefulRoute } from "./createStatefulRoute"
import UserDetailAttrs from "./users/UserDetailAttrs"
import UserDetailEvents from "./users/UserDetailEvents"
import UserDetailInbox from "./users/UserDetailInbox"
import UserDetailScheduled from "./users/UserDetailScheduled"
import UserDetailSubscriptions from "./users/UserDetailSubscriptions"
import Campaigns from "./campaign/Campaigns"
import Campaign from "./campaign/Campaign"
import CampaignDetails from "./campaign/CampaignDetails"
import NewCampaign from "./campaign/NewCampaign"
import Template from "./campaign/template/Template"
import TemplateContent from "./campaign/template/Content"
import TemplateReview from "./campaign/template/Review"
import EmailEditor from "./campaign/template/mail/editor/Editor"
// EmailBuilder is now embedded within the Editor view (not a separate route)
import Journeys from "./journey/Journeys"
import JourneyEditor from "./journey/editor/JourneyEditor"
import ProjectSettings from "./settings/ProjectSettings"
import Integrations from "./settings/Integrations"
import NewIntegration from "./settings/NewIntegration"
import IntegrationSetup from "./settings/IntegrationSetup"
import Login from "./auth/Login"
import LoginCallback from "./auth/LoginCallback"
import Register from "./auth/Register"
import ForgotPassword from "./auth/ForgotPassword"
import ResetPassword from "./auth/ResetPassword"
import VerifyEmail from "./auth/VerifyEmail"
import MyInvites from "./invites/MyInvites"
import Onboarding from "./auth/Onboarding"
import OnboardingProject from "./auth/OnboardingProject"
import {
    BuildingIcon,
    CampaignsIcon,
    CheckCircleIcon,
    IntegrationsIcon,
    JourneysIcon,
    ListsIcon,
    SettingsIcon,
    UsersIcon,
} from "@/components/icons"
import { Radio } from "lucide-react"
import Broadcasts from "./broadcast/Broadcasts"
import { BroadcastDetailRoute } from "./broadcast/BroadcastDetail"
import { Projects } from "./project/Projects"
import { completedGettingStarted } from "../utils"
import Settings from "./settings/Settings"
import ProjectSidebar from "./project/ProjectSidebar"
import ProjectGettingStarted from "./project/GettingStarted"
import ProjectOnboarding from "./project/ProjectOnboarding"
import ProjectOnboardingGettingStarted from "./project/ProjectOnboardingGettingStarted"
import ProjectOnboardingUsers from "./project/ProjectOnboardingUsers"
import ProjectOnboardingIntegration from "./project/ProjectOnboardingIntegration"
import ProjectOnboardingDomain from "./project/ProjectOnboardingDomain"
import Locales from "./settings/Locales"
import Domains from "./settings/Domains"
import { isEnterprise } from "@/config/enterprise"
import UserDetailJourneys from "./users/UserDetailJourneys"
import UserDetailOrganizations from "./users/UserDetailOrganizations"
import Organizations from "./organizations/Organizations"
import OrganizationDetail from "./organizations/OrganizationDetail"
import OrganizationDetailAttrs from "./organizations/OrganizationDetailAttrs"
import OrganizationDetailEvents from "./organizations/OrganizationDetailEvents"
import OrganizationDetailInbox from "./organizations/OrganizationDetailInbox"
import OrganizationDetailMembers from "./organizations/OrganizationDetailMembers"
import OrganizationDetailScheduled from "./organizations/OrganizationDetailScheduled"
import { Translation } from "react-i18next"
import type { UUID } from "@/types/common"
import type { Project } from "../types"
import type { SidebarLink } from "@/types/sidebar"

export interface RouterProps {
    routes?: (routes: RouteObject[]) => RouteObject[]
    projectSidebarLinks?: <T extends SidebarLink>(links: T[]) => T[]
}

export const createRouter = ({
    routes = (routes) => routes,
    projectSidebarLinks = (links) => links,
}: RouterProps) =>
    createBrowserRouter(
        routes([
            {
                path: "/login",
                element: <Login />,
            },
            {
                path: "/login/:driver/callback",
                element: <LoginCallback />,
            },
            {
                path: "/register",
                element: <Register />,
            },
            {
                path: "/forgot-password",
                element: <ForgotPassword />,
            },
            // Both of these are reached from an emailed link carrying a
            // single-use token in the query string.
            {
                path: "/reset-password",
                element: <ResetPassword />,
            },
            {
                path: "/verify-email",
                element: <VerifyEmail />,
            },
            {
                path: "/invites",
                element: <MyInvites />,
            },
            {
                path: "*",
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
                            const projects = await api.projects.all()
                            if (!projects || projects.results.length === 0) {
                                return redirect("onboarding/project")
                            }

                            const [first] = projects.results
                            return redirect(`projects/${first.id}`)
                        },
                        element: <Projects />,
                    },
                    {
                        path: "onboarding",
                        element: <Onboarding />,
                        children: [
                            {
                                path: "project",
                                element: <OnboardingProject />,
                            },
                        ],
                    },
                    {
                        path: "projects/:projectId",
                        loader: async ({ params: { projectId = "" } }) => {
                            const project = await api.projects.get(projectId)
                            return project
                        },
                        shouldRevalidate: () => true,
                        element: (
                            <StatefulLoaderContextProvider context={ProjectContext}>
                                <Outlet />
                            </StatefulLoaderContextProvider>
                        ),
                        children: [
                            {
                                path: "onboarding",
                                element: <ProjectOnboarding />,
                                children: [
                                    {
                                        index: true,
                                        path: "",
                                        loader: async ({ params: { projectId = "" } }) => {
                                            const project = await api.projects.get(projectId)
                                            if ((project.integrations_count ?? 0) > 0) {
                                                return redirect("users")
                                            }
                                        },
                                        element: <ProjectOnboardingIntegration />,
                                    },
                                    ...(isEnterprise
                                        ? [
                                              {
                                                  path: "domain",
                                                  loader: async ({
                                                      params: { projectId = "" },
                                                  }: {
                                                      params: { projectId?: string }
                                                  }) => {
                                                      const { hasCourierProvider } =
                                                          await import("@/utils")
                                                      const hasProvider =
                                                          await hasCourierProvider(projectId)
                                                      if (!hasProvider) {
                                                          return redirect("../users")
                                                      }
                                                      return null
                                                  },
                                                  element: <ProjectOnboardingDomain />,
                                              },
                                          ]
                                        : []),
                                    {
                                        path: "users",
                                        element: <ProjectOnboardingUsers />,
                                    },
                                    {
                                        path: "getting-started",
                                        element: <ProjectOnboardingGettingStarted />,
                                    },
                                ],
                            },
                            {
                                path: "",
                                loader: async ({ params: { projectId = "" } }) => {
                                    const project = await api.projects.get(projectId)
                                    if (
                                        !sessionStorage.getItem("skippedOnboarding") &&
                                        project.users_count === 0
                                    ) {
                                        return redirect("onboarding")
                                    }
                                },
                                element: (
                                    <ProjectSidebar
                                        links={projectSidebarLinks([
                                            {
                                                key: "getting-started",
                                                to: "getting-started",
                                                children: (
                                                    <Translation>
                                                        {(t) => t("getting-started")}
                                                    </Translation>
                                                ),
                                                icon: <CheckCircleIcon />,
                                                active: (project: Project) => {
                                                    return !completedGettingStarted(project)
                                                },
                                                minRole: "editor",
                                            },
                                            {
                                                key: "campaigns",
                                                to: "campaigns",
                                                children: (
                                                    <Translation>
                                                        {(t) => t("campaign.plural")}
                                                    </Translation>
                                                ),
                                                icon: <CampaignsIcon />,
                                                minRole: "editor",
                                            },
                                            ...(isEnterprise
                                                ? [
                                                      {
                                                          key: "broadcasts",
                                                          to: "broadcasts",
                                                          children: (
                                                              <Translation>
                                                                  {(t) =>
                                                                      t("broadcasts", "Broadcasts")
                                                                  }
                                                              </Translation>
                                                          ),
                                                          icon: <Radio className="h-4 w-4" />,
                                                          minRole: "editor" as const,
                                                      },
                                                  ]
                                                : []),
                                            {
                                                key: "journeys",
                                                to: "journeys",
                                                children: (
                                                    <Translation>
                                                        {(t) => t("journeys")}
                                                    </Translation>
                                                ),
                                                icon: <JourneysIcon />,
                                                minRole: "editor",
                                            },
                                            {
                                                key: "organizations",
                                                to: "organizations",
                                                children: (
                                                    <Translation>
                                                        {(t) => t("organizations")}
                                                    </Translation>
                                                ),
                                                icon: <BuildingIcon />,
                                            },
                                            {
                                                key: "users",
                                                to: "users",
                                                children: (
                                                    <Translation>{(t) => t("users")}</Translation>
                                                ),
                                                icon: <UsersIcon />,
                                            },
                                            {
                                                key: "lists",
                                                to: "lists",
                                                children: (
                                                    <Translation>{(t) => t("lists")}</Translation>
                                                ),
                                                icon: <ListsIcon />,
                                                minRole: "editor",
                                            },
                                            {
                                                key: "integrations",
                                                to: "integrations",
                                                children: (
                                                    <Translation>
                                                        {(t) => t("integrations")}
                                                    </Translation>
                                                ),
                                                icon: <IntegrationsIcon />,
                                                minRole: "admin",
                                            },
                                            {
                                                key: "settings",
                                                to: "settings",
                                                children: (
                                                    <Translation>
                                                        {(t) => t("settings")}
                                                    </Translation>
                                                ),
                                                icon: <SettingsIcon />,
                                                minRole: "admin",
                                            },
                                        ])}
                                    >
                                        <Outlet />
                                    </ProjectSidebar>
                                ),
                                children: [
                                    {
                                        index: true,
                                        loader: async ({ params: { projectId = "" } }) => {
                                            const project = await api.projects.get(projectId)
                                            if (project.role === "support") {
                                                return redirect(`/projects/${project.id}/users`)
                                            }
                                            return redirect("campaigns")
                                        },
                                    },
                                    {
                                        path: "getting-started",
                                        loader: async ({ params: { projectId = "" } }) => {
                                            const project = await api.projects.get(projectId)
                                            if (completedGettingStarted(project)) {
                                                return redirect(`/projects/${projectId}/campaigns`)
                                            }
                                            return null
                                        },
                                        element: <ProjectGettingStarted />,
                                    },
                                    {
                                        path: "campaigns",
                                        children: [
                                            createStatefulRoute({
                                                path: "",
                                                apiPath: api.campaigns,
                                                element: <Campaigns />,
                                            }),
                                            createStatefulRoute({
                                                path: "new",
                                                apiPath: api.campaigns,
                                                element: <Campaigns create={true} />,
                                            }),
                                            {
                                                path: "new/:channel",
                                                element: <NewCampaign />,
                                                errorElement: <ErrorPage />,
                                            },
                                            createStatefulRoute({
                                                path: ":entityId",
                                                apiPath: api.campaigns,
                                                paramName: "entityId",
                                                context: CampaignContext,
                                                element: <Campaign />,
                                                children: [
                                                    {
                                                        index: true,
                                                        element: <CampaignDetails />,
                                                    },
                                                    {
                                                        path: "templates/:templateId",
                                                        loader: async ({ params }) => {
                                                            const projectId = params.projectId as
                                                                | UUID
                                                                | undefined
                                                            const campaignId = params.entityId as
                                                                | UUID
                                                                | undefined
                                                            const templateId = params.templateId as
                                                                | UUID
                                                                | undefined

                                                            if (!projectId || !campaignId) {
                                                                throw new Error("Not Found")
                                                            }

                                                            if (!templateId) {
                                                                return await api.campaigns.templates.search(
                                                                    projectId,
                                                                    campaignId,
                                                                    { limit: 20 },
                                                                )
                                                            }

                                                            return await api.campaigns.templates.get(
                                                                projectId,
                                                                campaignId,
                                                                templateId,
                                                            )
                                                        },
                                                        element: (
                                                            <StatefulLoaderContextProvider
                                                                context={TemplateContext}
                                                            >
                                                                <Template />
                                                            </StatefulLoaderContextProvider>
                                                        ),
                                                        errorElement: <ErrorPage />,
                                                        children: [
                                                            {
                                                                index: true,
                                                                element: <TemplateContent />,
                                                            },
                                                            {
                                                                path: "email/editor",
                                                                element: <EmailEditor />,
                                                            },
                                                            {
                                                                path: "review",
                                                                element: <TemplateReview />,
                                                            },
                                                        ],
                                                    },
                                                ],
                                            }),
                                        ],
                                    },
                                    ...(isEnterprise
                                        ? [
                                              {
                                                  path: "broadcasts",
                                                  element: <Broadcasts />,
                                              },
                                              {
                                                  path: "broadcasts/:broadcastId",
                                                  element: <BroadcastDetailRoute />,
                                              },
                                          ]
                                        : []),
                                    createStatefulRoute({
                                        path: "journeys",
                                        apiPath: api.journeys,
                                        element: <Journeys />,
                                    }),
                                    createStatefulRoute({
                                        path: "journeys/:entityId",
                                        apiPath: api.journeys,
                                        paramName: "entityId",
                                        context: JourneyContext,
                                        element: <Outlet />,
                                        children: [
                                            {
                                                index: true,
                                                element: <JourneyEditor />,
                                            },
                                        ],
                                    }),
                                    createStatefulRoute({
                                        path: "users",
                                        apiPath: api.users,
                                        element: <Users />,
                                    }),
                                    createStatefulRoute({
                                        path: "users/:entityId",
                                        apiPath: api.users,
                                        paramName: "entityId",
                                        context: UserContext,
                                        element: <UserDetail />,
                                        children: [
                                            {
                                                index: true,
                                                element: <UserDetailAttrs />,
                                            },
                                            {
                                                path: "events",
                                                element: <UserDetailEvents />,
                                            },
                                            {
                                                path: "scheduled",
                                                element: <UserDetailScheduled />,
                                            },
                                            {
                                                path: "inbox",
                                                element: <UserDetailInbox />,
                                            },
                                            {
                                                path: "subscriptions",
                                                element: <UserDetailSubscriptions />,
                                            },
                                            {
                                                path: "journeys",
                                                element: <UserDetailJourneys />,
                                            },
                                            {
                                                path: "organizations",
                                                element: <UserDetailOrganizations />,
                                            },
                                        ],
                                    }),
                                    createStatefulRoute({
                                        path: "lists",
                                        apiPath: api.lists,
                                        element: <Lists />,
                                    }),
                                    createStatefulRoute({
                                        path: "lists/:entityId",
                                        apiPath: api.lists,
                                        paramName: "entityId",
                                        context: ListContext,
                                        element: <ListDetail />,
                                    }),
                                    {
                                        path: "organizations",
                                        element: <Organizations />,
                                    },
                                    {
                                        path: "organizations/:organizationId",
                                        loader: async ({ params }) => {
                                            const { data, error } = await oapiClient.GET(
                                                "/api/admin/projects/{projectID}/subjects/organizations/{organizationID}",
                                                {
                                                    params: {
                                                        path: {
                                                            projectID: params.projectId!,
                                                            organizationID: params.organizationId!,
                                                        },
                                                    },
                                                },
                                            )
                                            if (error) throw new Error("Organization not found")
                                            return data
                                        },
                                        element: (
                                            <StatefulLoaderContextProvider
                                                context={OrganizationContext}
                                            >
                                                <OrganizationDetail />
                                            </StatefulLoaderContextProvider>
                                        ),
                                        children: [
                                            {
                                                index: true,
                                                element: <OrganizationDetailAttrs />,
                                            },
                                            {
                                                path: "events",
                                                element: <OrganizationDetailEvents />,
                                            },
                                            {
                                                path: "scheduled",
                                                element: <OrganizationDetailScheduled />,
                                            },
                                            {
                                                path: "members",
                                                element: <OrganizationDetailMembers />,
                                            },
                                            {
                                                path: "inbox",
                                                element: <OrganizationDetailInbox />,
                                            },
                                        ],
                                    },
                                    {
                                        path: "integrations",
                                        element: <Integrations />,
                                    },
                                    {
                                        path: "integrations/new",
                                        element: <NewIntegration />,
                                    },
                                    {
                                        path: "integrations/new/:module",
                                        element: <IntegrationSetup />,
                                    },
                                    {
                                        path: "integrations/new/:kind/:module",
                                        element: <IntegrationSetup />,
                                    },
                                    {
                                        path: "integrations/:kind/:id",
                                        element: <IntegrationSetup />,
                                    },
                                    {
                                        path: "integrations/:id",
                                        element: <IntegrationSetup />,
                                    },
                                    {
                                        path: "settings",
                                        element: <Settings />,
                                        children: [
                                            {
                                                index: true,
                                                element: <ProjectSettings />,
                                            },
                                            {
                                                path: "locales",
                                                element: <Locales />,
                                            },
                                            {
                                                path: "access",
                                                children: [
                                                    { index: true, element: <ClientList /> },
                                                    { path: "new", element: <NewClientRoute /> },
                                                    {
                                                        path: ":clientId",
                                                        element: <EditClientRoute />,
                                                    },
                                                ],
                                            },
                                            {
                                                path: "members",
                                                element: <Members />,
                                            },
                                            {
                                                path: "invites",
                                                element: <Navigate to="../members" replace />,
                                            },
                                            {
                                                path: "subscriptions",
                                                element: <Subscriptions />,
                                            },
                                            {
                                                path: "event-schemas",
                                                element: <EventSchemas />,
                                            },
                                            {
                                                path: "push-providers",
                                                element: <PushProviders />,
                                            },
                                            ...(isEnterprise
                                                ? [
                                                      {
                                                          path: "domains",
                                                          element: <Domains />,
                                                      },
                                                  ]
                                                : []),
                                        ],
                                    },
                                ],
                            },
                        ],
                    },
                    {
                        path: "*",
                        element: <ErrorPage status={404} />,
                        errorElement: <ErrorPage />,
                    },
                ],
            },
        ]),
    )
