import { Outlet, NavLink, useLocation } from 'react-router'
import { useTranslation } from 'react-i18next'
import { useContext } from 'react'
import {
    Settings as SettingsLucideIcon,
    Globe,
    Key,
    Puzzle,
    Bell,
} from 'lucide-react'
import { ProjectContext } from '../../contexts'
import { ProjectRoleRequired } from '../project/ProjectRoleRequired'
import { SettingsIcon } from '@/components/icons'
import { cn } from '../../utils'

export default function Settings() {
    const { t } = useTranslation()
    const [project] = useContext(ProjectContext)

    const basePath = `/projects/${project.id}/settings`
    const location = useLocation()
    const currentPath = location.pathname
    const activeTab = currentPath === basePath ? 'general' : currentPath.split('/').pop()

    const tabs = [
        { key: 'general', to: '', label: t('general'), icon: SettingsLucideIcon },
        { key: 'locales', to: 'locales', label: t('locales'), icon: Globe },
        { key: 'api-keys', to: 'api-keys', label: t('api_keys'), icon: Key },
        { key: 'integrations', to: 'integrations', label: t('integrations'), icon: Puzzle },
        { key: 'subscriptions', to: 'subscriptions', label: t('subscriptions'), icon: Bell },
    ]

    return (
        <ProjectRoleRequired minRole="admin">
            <div className="flex flex-col min-h-full">
                {/* Header Section */}
                <div className="border-b bg-card/50">
                    <div className="p-6 pb-0">
                        <div className="flex items-start gap-4">
                            <div className="flex h-14 w-14 items-center justify-center rounded-xl shrink-0 bg-muted [&>svg]:h-7 [&>svg]:w-7 [&>svg]:text-muted-foreground">
                                <SettingsIcon />
                            </div>
                            <div className="space-y-1">
                                <h1 className="text-2xl font-semibold tracking-tight">
                                    {t('settings')}
                                </h1>
                                <p className="text-sm text-muted-foreground">
                                    {t('settings_description', 'Manage your project configuration, integrations, and preferences.')}
                                </p>
                            </div>
                        </div>

                        {/* Navigation Tabs */}
                        <nav className="flex gap-1 mt-6 -mb-px">
                            {tabs.map((tab) => {
                                const Icon = tab.icon
                                const isActive = activeTab === tab.key
                                return (
                                    <NavLink
                                        key={tab.key}
                                        to={tab.to}
                                        end={tab.to === ''}
                                        className={cn(
                                            'flex items-center gap-2 px-4 py-2.5 text-sm font-medium rounded-t-lg border-b-2 transition-colors',
                                            isActive
                                                ? 'border-primary text-foreground bg-background'
                                                : 'border-transparent text-muted-foreground hover:text-foreground hover:bg-muted/50',
                                        )}
                                    >
                                        <Icon className="h-4 w-4" />
                                        {tab.label}
                                    </NavLink>
                                )
                            })}
                        </nav>
                    </div>
                </div>

                {/* Content Area */}
                <div className="flex-1 p-6">
                    <Outlet />
                </div>
            </div>
        </ProjectRoleRequired>
    )
}
