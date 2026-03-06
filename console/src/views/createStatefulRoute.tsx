import type { Context } from "react"
import type { RouteObject } from "react-router"
import type { ProjectEntityPath } from "../api"
import type { UseStateContext } from "../types"
import ErrorPage from "./ErrorPage"
import { StatefulLoaderContextProvider } from "./LoaderContextProvider"
import type { UUID } from "@/types/common"

interface StatefulRoute<T extends Record<string, unknown>> {
    context?: Context<UseStateContext<T>>
    apiPath: ProjectEntityPath<T>
    path: string
    element?: JSX.Element
    children?: Array<RouteObject & { tab?: string }>
    paramName?: string
}

export function createStatefulRoute<T extends { id: UUID }>({
    context,
    path,
    apiPath,
    element,
    children = [],
    paramName = "entityId",
}: StatefulRoute<T>): RouteObject {
    return {
        path,
        loader: async ({ params }) => {
            const projectId = params.projectId as UUID | undefined
            if (!projectId) {
                throw new Error("Not Found")
            }

            if (!paramName || !params[paramName]) {
                return await apiPath.search(projectId, { limit: 20 })
            }

            return await apiPath.get(projectId, params[paramName] as UUID)
        },
        element: context ? (
            <StatefulLoaderContextProvider key={path} context={context}>
                {element}
            </StatefulLoaderContextProvider>
        ) : (
            element
        ),
        children: children.map(({ ...rest }) => rest),
        errorElement: <ErrorPage />,
    }
}
