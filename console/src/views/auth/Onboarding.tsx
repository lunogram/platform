import { Outlet } from 'react-router'
import './Auth.css'

export default function Onboarding() {
    return (
        <div className="auth onboarding">
            <div className="logo">
                {/* <Logo /> */}
            </div>
            <Outlet />
        </div>
    )
}
