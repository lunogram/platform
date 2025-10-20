import { Outlet, useLoaderData } from 'react-router'
import { AdminContext } from '../contexts'
import type { Admin } from '../types'

export default function Auth() {
    return (
        <AdminContext.Provider value={useLoaderData<Admin>()}>
            <Outlet />
        </AdminContext.Provider>
    )
}
