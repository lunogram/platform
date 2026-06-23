import { useContext, useState } from "react"
import { Outlet, useNavigate, useLocation, Link } from "react-router"
import { useTranslation } from "react-i18next"
import {
    Building2,
    Trash2,
    FileText,
    Users,
    Activity,
    ChevronRight,
    Inbox,
    MoreHorizontal,
} from "lucide-react"
import { ProjectContext, OrganizationContext } from "../../contexts"
import { PreferencesContext } from "@/contexts/PreferencesContext"
import { getRandomColor } from "@/lib/colors"
import { getPrimaryExternalId } from "@/lib/name"
import { formatDate } from "../../utils"
import oapiClient from "../../oapi/client"
import { toast } from "sonner"

import { Button } from "@/components/ui/button"
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from "@/components/ui/dialog"
import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { InlineEdit } from "@/components/ui/inline-edit"
import { NavTabs } from "@/components/ui/nav-tabs"

export default function OrganizationDetail() {
    const { t } = useTranslation()
    const navigate = useNavigate()
    const location = useLocation()
    const [preferences] = useContext(PreferencesContext)
    const [project] = useContext(ProjectContext)
    const [organization, setOrganization] = useContext(OrganizationContext)
    const [isDeleteOpen, setIsDeleteOpen] = useState(false)
    const [isDeleting, setIsDeleting] = useState(false)

    const orgColor = getRandomColor(
        getPrimaryExternalId(organization as unknown as Record<string, unknown>) ?? organization.id,
    )

    // Determine active tab
    const basePath = `/projects/${project.id}/organizations/${organization.id}`
    const currentPath = location.pathname
    const activeTab = currentPath === basePath ? "details" : currentPath.replace(basePath + "/", "").split("/")[0]

    const deleteOrganization = async () => {
        setIsDeleting(true)
        try {
            await oapiClient.DELETE(
                "/api/admin/projects/{projectID}/subjects/organizations/{organizationID}",
                {
                    params: {
                        path: {
                            projectID: project.id,
                            organizationID: organization.id,
                        },
                    },
                },
            )
            await navigate(`/projects/${project.id}/organizations`)
        } finally {
            setIsDeleting(false)
        }
    }

    const tabs = [
        { key: "details", to: "", label: t("details"), icon: FileText },
        { key: "inbox", to: "inbox", label: t("inbox", "Inbox"), icon: Inbox },
        { key: "members", to: "members", label: t("members"), icon: Users },
        { key: "events", to: "events", label: t("events"), icon: Activity },
    ]

    return (
        <div className="flex flex-col min-h-full">
            {/* Header Section */}
            <div className="border-b bg-card/50">
                <div className="p-4 sm:p-6 pb-0 sm:pb-0">
                    {/* Breadcrumb */}
                    <nav className="flex items-center gap-1.5 text-sm text-muted-foreground mb-4">
                        <Link
                            to={`/projects/${project.id}/organizations`}
                            className="hover:text-foreground transition-colors"
                        >
                            {t("organizations")}
                        </Link>
                        <ChevronRight className="h-3.5 w-3.5" />
                        <span className="text-foreground font-medium">
                            {organization.name ||
                                getPrimaryExternalId(
                                    organization as unknown as Record<string, unknown>,
                                )}
                        </span>
                    </nav>

                    {/* Organization Identity */}
                    <div className="flex items-start justify-between gap-6">
                        <div className="flex items-start gap-4 min-w-0">
                            <div
                                className="flex h-14 w-14 items-center justify-center rounded-xl shrink-0"
                                style={{ backgroundColor: orgColor }}
                            >
                                <Building2 className="h-7 w-7 text-white" />
                            </div>
                            <div className="space-y-1 min-w-0">
                                <InlineEdit
                                    value={organization.name ?? ""}
                                    onSave={async (name) => {
                                        const { data } = await oapiClient.PATCH(
                                            "/api/admin/projects/{projectID}/subjects/organizations/{organizationID}",
                                            {
                                                params: {
                                                    path: {
                                                        projectID: project.id,
                                                        organizationID: organization.id,
                                                    },
                                                },
                                                body: { name },
                                            },
                                        )
                                        if (data) {
                                            setOrganization(data)
                                            toast.success(
                                                t("organization_updated", "Organization updated"),
                                            )
                                        }
                                    }}
                                    placeholder={getPrimaryExternalId(
                                        organization as unknown as Record<string, unknown>,
                                    )}
                                    triggerClassName="gap-1.5"
                                    pencilSize="h-3.5 w-3.5"
                                >
                                    <h1 className="text-2xl font-semibold tracking-tight">
                                        {organization.name ||
                                            getPrimaryExternalId(
                                                organization as unknown as Record<string, unknown>,
                                            )}
                                    </h1>
                                </InlineEdit>
                                <p className="text-sm text-muted-foreground">
                                    <span>
                                        Created{" "}
                                        {formatDate(preferences, organization.created_at, "PP")}
                                    </span>
                                </p>
                            </div>
                        </div>

                        <DropdownMenu>
                            <DropdownMenuTrigger asChild>
                                <Button variant="ghost" size="icon" className="h-8 w-8">
                                    <MoreHorizontal className="h-4 w-4" />
                                </Button>
                            </DropdownMenuTrigger>
                            <DropdownMenuContent align="end">
                                <DropdownMenuItem
                                    className="text-destructive focus:text-destructive"
                                    onClick={() => setIsDeleteOpen(true)}
                                >
                                    <Trash2 className="h-4 w-4 mr-2" />
                                    {t("delete")}
                                </DropdownMenuItem>
                            </DropdownMenuContent>
                        </DropdownMenu>
                    </div>

                    {/* Navigation Tabs - Integrated with header */}
                    <NavTabs tabs={tabs} activeTab={activeTab} className="mt-6" />
                </div>
            </div>

            {/* Content Area */}
            <div className="flex-1 p-4 sm:p-6">
                <Outlet />
            </div>

            {/* Delete Confirmation Dialog */}
            <Dialog open={isDeleteOpen} onOpenChange={setIsDeleteOpen}>
                <DialogContent>
                    <DialogHeader>
                        <DialogTitle>{t("delete_organization")}</DialogTitle>
                        <DialogDescription>
                            {t(
                                "delete_organization_warning",
                                "Are you sure you want to delete this organization? This action cannot be undone.",
                            )}
                        </DialogDescription>
                    </DialogHeader>
                    <div className="py-4">
                        <div className="flex items-center gap-3 p-3 rounded-lg bg-muted">
                            <div
                                className="flex h-10 w-10 items-center justify-center rounded-lg shrink-0"
                                style={{ backgroundColor: orgColor }}
                            >
                                <Building2 className="h-5 w-5 text-white" />
                            </div>
                            <div>
                                <p className="font-medium">
                                    {organization.name ||
                                        getPrimaryExternalId(
                                            organization as unknown as Record<string, unknown>,
                                        )}
                                </p>
                                <p className="text-sm text-muted-foreground">
                                    {getPrimaryExternalId(
                                        organization as unknown as Record<string, unknown>,
                                    )}
                                </p>
                            </div>
                        </div>
                    </div>
                    <DialogFooter>
                        <Button
                            variant="outline"
                            onClick={() => setIsDeleteOpen(false)}
                            disabled={isDeleting}
                        >
                            {t("cancel")}
                        </Button>
                        <Button
                            variant="destructive"
                            onClick={deleteOrganization}
                            disabled={isDeleting}
                        >
                            {isDeleting ? t("deleting") : t("delete_organization")}
                        </Button>
                    </DialogFooter>
                </DialogContent>
            </Dialog>
        </div>
    )
}
