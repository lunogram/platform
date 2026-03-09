import { Outlet, useLocation } from "react-router"
import { useTranslation } from "react-i18next"
import { useContext } from "react"
import { Settings as SettingsLucideIcon, Globe, Key, Bell, Zap } from "lucide-react"
import { ProjectContext } from "../../contexts"
import { ProjectRoleRequired } from "../project/ProjectRoleRequired"
import { SettingsIcon } from "@/components/icons"
import { NavTabs } from "@/components/ui/nav-tabs"

export default function Settings() {
    const { t } = useTranslation()
    const [project] = useContext(ProjectContext)

    const basePath = `/projects/${project.id}/settings`
    const location = useLocation()
    const currentPath = location.pathname
    const activeTab = currentPath === basePath ? "general" : currentPath.split("/").pop()

    const tabs = [
        { key: "general", to: "", label: t("general"), icon: SettingsLucideIcon },
        { key: "locales", to: "locales", label: t("locales"), icon: Globe },
        { key: "api-keys", to: "api-keys", label: t("api_keys"), icon: Key },
        { key: "subscriptions", to: "subscriptions", label: t("subscriptions"), icon: Bell },
        {
            key: "event-schemas",
            to: "event-schemas",
            label: t("event_schemas", "Event Schemas"),
            icon: Zap,
        },
    ]

    return (
        <ProjectRoleRequired minRole="admin">
            <div className="flex flex-col min-h-full">
                {/* Header Section */}
                <div className="border-b bg-card/50">
                    <div className="p-4 sm:p-6 pb-0 sm:pb-0">
                        <div className="flex items-start gap-3 sm:gap-4">
                            <div className="flex h-10 w-10 sm:h-14 sm:w-14 items-center justify-center rounded-xl shrink-0 bg-muted [&>svg]:h-5 [&>svg]:w-5 sm:[&>svg]:h-7 sm:[&>svg]:w-7 [&>svg]:text-muted-foreground">
                                <SettingsIcon />
                            </div>
                            <div className="space-y-1">
                                <h1 className="text-xl sm:text-2xl font-semibold tracking-tight">
                                    {t("settings")}
                                </h1>
                                <p className="text-sm text-muted-foreground hidden sm:block">
                                    {t(
                                        "settings_description",
                                        "Manage your project configuration, integrations, and preferences.",
                                    )}
                                </p>
                            </div>
                        </div>

                        {/* Navigation Tabs */}
                        <NavTabs tabs={tabs} activeTab={activeTab} className="mt-4 sm:mt-6" />
                    </div>
                </div>

                {/* Content Area */}
                <div className="flex-1 p-4 sm:p-6">
                    <Outlet />
                </div>
            </div>
        </ProjectRoleRequired>
    )
}
