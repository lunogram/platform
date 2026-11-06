import type { ReactNode } from "react"
import { QueryClientProvider } from "@tanstack/react-query"
import { render } from "@testing-library/react"
import { createMemoryRouter, RouterProvider, type RouteObject } from "react-router"
import { OrganizationContext, ProjectContext } from "@/contexts"
import { queryClient } from "@/lib/query"
import type { oapiClient, Organization } from "@/oapi/client"
import type { Project } from "@/types"

// Shared fixtures and setup for the organizations tests. Each test file still
// mocks its own modules, since vi.mock only applies to the file it is in.

export const project = { id: "p1", name: "Acme", locale: "en" } as Project

export const organization = {
    id: "o1",
    project_id: "p1",
    name: "Acme",
    data: {},
    created_at: "2026-10-01T10:00:00Z",
    updated_at: "2026-10-01T10:00:00Z",
} as unknown as Organization

export const organizationsPath = "/projects/p1/organizations"
export const organizationPath = `${organizationsPath}/o1`

type ClientResult = Awaited<ReturnType<typeof oapiClient.GET>>
export const ok = (data: unknown) => ({ data }) as unknown as ClientResult
export const failed = (detail: string) => ({ error: { detail } }) as unknown as ClientResult

/** Stubs the browser APIs Radix needs that jsdom lacks, and turns off retries. */
export function setUpDom() {
    Element.prototype.hasPointerCapture = () => false
    Element.prototype.releasePointerCapture = () => {}
    Element.prototype.scrollIntoView = () => {}
    globalThis.ResizeObserver ??= class {
        observe() {}
        unobserve() {}
        disconnect() {}
    } as unknown as typeof ResizeObserver
    // Show a failed request right away instead of retrying it first.
    queryClient.setDefaultOptions({ queries: { retry: false } })
}

/** Renders routes inside the providers every organizations page needs. */
export function renderRoutes(routes: RouteObject[], initialPath: string) {
    const router = createMemoryRouter(routes, { initialEntries: [initialPath] })
    render(
        <QueryClientProvider client={queryClient}>
            <ProjectContext.Provider value={[project, () => {}]}>
                <RouterProvider router={router} />
            </ProjectContext.Provider>
        </QueryClientProvider>,
    )
    return router
}

/** Renders one tab of the organization page with the organization in context. */
export function renderOrganizationTab(tab: ReactNode) {
    return renderRoutes(
        [
            {
                path: "/projects/:projectId/organizations/:organizationId",
                element: (
                    <OrganizationContext.Provider value={[organization, () => {}]}>
                        {tab}
                    </OrganizationContext.Provider>
                ),
            },
            { path: "/projects/:projectId/users/:userId", element: <p>user page</p> },
        ],
        organizationPath,
    )
}
