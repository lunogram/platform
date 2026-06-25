import { Outlet, useLocation } from "react-router"
import { useTranslation } from "react-i18next"
import { Activity, CalendarClock } from "lucide-react"
import { NavTabs } from "@/components/ui/nav-tabs"

export default function EventsLayout() {
    const { t } = useTranslation()
    const location = useLocation()

    const tabs = [
        { key: "activity", to: "", label: t("activity", "Activity"), icon: Activity },
        {
            key: "scheduled",
            to: "scheduled",
            label: t("scheduled", "Scheduled"),
            icon: CalendarClock,
        },
    ]

    const activeTab = location.pathname.endsWith("/scheduled") ? "scheduled" : "activity"

    return (
        <div>
            <div className="-mx-4 sm:-mx-6 -mt-4 sm:-mt-6 px-4 sm:px-6 border-b bg-card/50 mb-6">
                <NavTabs tabs={tabs} activeTab={activeTab} />
            </div>
            <Outlet />
        </div>
    )
}
