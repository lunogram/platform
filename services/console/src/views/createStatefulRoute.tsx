import type { Context } from 'react'
import type { RouteObject } from 'react-router'
import type { ProjectEntityPath } from '../api'
import type { UseStateContext } from '../types'
import ErrorPage from './ErrorPage'
import { StatefulLoaderContextProvider } from './LoaderContextProvider'
import { NIL } from 'uuid'
import type { UUID } from '@/types/common'

interface StatefulRoute<T extends Record<string, unknown>> {
    context?: Context<UseStateContext<T>>
    apiPath: ProjectEntityPath<T>
    path: string
    element?: JSX.Element
    children?: Array<RouteObject & { tab?: string }>
}

export function createStatefulRoute<T extends { id: UUID }>({ context, path, apiPath, element, children = [] }: StatefulRoute<T>): RouteObject {
    return {
        path,
        loader: async ({ params: { projectId = NIL, entityId = NIL } }) => {
            if (projectId === NIL) {
                throw new Error('Not Found')
            }

            if (entityId === NIL) {
                return await apiPath.search(projectId as UUID, { limit: 20 })
            }

            return await apiPath.get(projectId as UUID, entityId as UUID)
        },
        element: context
            ? (
                <StatefulLoaderContextProvider key={path} context={context}>
                    {element}
                </StatefulLoaderContextProvider>
            )
            : element,
        children: children.map(({ ...rest }) => rest),
        errorElement: <ErrorPage />,
    }
}
