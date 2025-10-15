import { Outlet } from 'react-router'

import './Onboarding.css'

export default function ProjectOnboarding() {
    return (
        <div className="auth onboarding">
            <div className="logo">
                {/* <Logo /> */}
            </div>
            <div className="onboarding-step">
                <Outlet />
            </div>
        </div>
    )
}
