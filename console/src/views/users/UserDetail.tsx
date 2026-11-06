import { useContext, useState } from "react"
import { Outlet, useNavigate, useLocation, Link } from "react-router"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"
import {
    Trash2,
    FileText,
    Activity,
    CalendarClock,
    Bell,
    Route,
    Building2,
    ChevronRight,
    MoreHorizontal,
    Mail,
    Phone,
    Inbox,
} from "lucide-react"
import { ProjectContext, UserContext } from "../../contexts"
import { PreferencesContext } from "@/contexts/PreferencesContext"
import { getRandomColor } from "@/lib/colors"
import { getUserDisplayName, getUserInitials, getPrimaryExternalId } from "@/lib/name"
import { formatDate } from "../../utils"
import { getTimezoneCoordinates } from "@/lib/timezone-coordinates"

import { Button } from "@/components/ui/button"
import { Map, MapMarker, MarkerContent } from "@/components/ui/map"
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
import { NavTabs } from "@/components/ui/nav-tabs"
import { InlineEdit } from "@/components/ui/inline-edit"
import { optionalPhoneSchema } from "@/validation/phone"
import { useDeleteUser, useUpdateUser } from "./queries"
import { UserLocaleEdit } from "./UserLocaleEdit"
import { UserTimezoneEdit } from "./UserTimezoneEdit"

export default function UserDetail() {
    const { t } = useTranslation()
    const navigate = useNavigate()
    const location = useLocation()
    const [preferences] = useContext(PreferencesContext)
    const [project] = useContext(ProjectContext)
    const [user] = useContext(UserContext)
    const [isDeleteOpen, setIsDeleteOpen] = useState(false)
    const deleteUserMutation = useDeleteUser()
    const isDeleting = deleteUserMutation.isPending
    const updateEmail = useUpdateUser(t("email_update_error", "Couldn't update the email"))
    const updatePhone = useUpdateUser(t("phone_update_error", "Couldn't update the phone"))

    // InlineEdit does not catch a rejected save, so a throw here would go
    // unhandled. The mutation cache has already shown the error.
    const saveEmail = async (value: string) => {
        try {
            await updateEmail.mutateAsync({ email: value || undefined })
            toast.success(t("email_updated", "Email updated"))
        } catch {
            // Reported by the mutation cache.
        }
    }

    const savePhone = async (value: string) => {
        try {
            await updatePhone.mutateAsync({ phone: value || undefined })
            toast.success(t("phone_updated", "Phone updated"))
        } catch {
            // Reported by the mutation cache.
        }
    }

    const userColor = getRandomColor(
        user.email ?? getPrimaryExternalId(user as unknown as Record<string, unknown>) ?? user.id,
    )

    const displayName = getUserDisplayName(user, "No name")

    const initials = getUserInitials(user)

    // Determine active tab
    const basePath = `/projects/${project.id}/users/${user.id}`
    const currentPath = location.pathname
    const activeTab =
        currentPath === basePath ? "details" : currentPath.replace(basePath + "/", "").split("/")[0]

    const deleteUser = () => {
        // The dialog stays open on failure; the mutation cache reports the error.
        deleteUserMutation.mutate(undefined, {
            onSuccess: () => navigate(`/projects/${project.id}/users`),
        })
    }

    const tabs = [
        { key: "details", to: "", label: t("details"), icon: FileText },
        { key: "inbox", to: "inbox", label: t("inbox", "Inbox"), icon: Inbox },
        { key: "events", to: "events", label: t("events"), icon: Activity },
        {
            key: "scheduled",
            to: "scheduled",
            label: t("scheduled", "Scheduled"),
            icon: CalendarClock,
        },
        { key: "subscriptions", to: "subscriptions", label: t("subscriptions"), icon: Bell },
        { key: "journeys", to: "journeys", label: t("journeys"), icon: Route },
        { key: "organizations", to: "organizations", label: t("organizations"), icon: Building2 },
    ]

    return (
        <div className="flex flex-col min-h-full">
            {/* Header Section */}
            <div className="border-b bg-card/50 relative overflow-hidden">
                {/* Ambient timezone map — faded right-side background */}
                {user.timezone &&
                    (() => {
                        const coordinates = getTimezoneCoordinates(user.timezone)
                        if (!coordinates) return null
                        return (
                            <div
                                className="ambient-map absolute inset-y-0 left-[50%] xl:left-[30%] right-0 hidden lg:block pointer-events-none overflow-hidden opacity-[0.45] dark:opacity-[0.35]"
                                style={{
                                    maskImage:
                                        "linear-gradient(to right, transparent 0%, black 40%)",
                                    WebkitMaskImage:
                                        "linear-gradient(to right, transparent 0%, black 40%)",
                                }}
                            >
                                <Map
                                    key={user.timezone}
                                    center={coordinates}
                                    zoom={3}
                                    interactive={false}
                                    theme="light"
                                    className="h-full w-full"
                                >
                                    <MapMarker longitude={coordinates[0]} latitude={coordinates[1]}>
                                        <MarkerContent>
                                            <div className="flex h-5 w-5 items-center justify-center rounded-full bg-primary/80 shadow-md">
                                                <div className="h-2 w-2 rounded-full bg-white" />
                                            </div>
                                        </MarkerContent>
                                    </MapMarker>
                                </Map>
                            </div>
                        )
                    })()}

                <div className="p-4 sm:p-6 pb-0 sm:pb-0 relative z-20">
                    {/* Breadcrumb */}
                    <nav className="flex items-center gap-1.5 text-sm text-muted-foreground mb-4">
                        <Link
                            to={`/projects/${project.id}/users`}
                            className="hover:text-foreground transition-colors"
                        >
                            {t("users")}
                        </Link>
                        <ChevronRight className="h-3.5 w-3.5" />
                        <span className="text-foreground font-medium">{displayName}</span>
                    </nav>

                    {/* User Identity */}
                    <div className="flex items-start justify-between gap-6">
                        <div className="flex items-start gap-4 min-w-0">
                            <div
                                className="flex h-14 w-14 items-center justify-center rounded-xl shrink-0 text-white text-lg font-medium"
                                style={{ backgroundColor: userColor }}
                            >
                                {initials}
                            </div>
                            <div className="space-y-1 min-w-0">
                                <h1 className="text-2xl font-semibold tracking-tight">
                                    {displayName}
                                </h1>
                                <p className="text-sm text-muted-foreground flex items-center flex-wrap gap-x-0">
                                    {/* 1. Email */}
                                    <InlineEdit
                                        value={user.email ?? ""}
                                        onSave={saveEmail}
                                        icon={<Mail className="h-3 w-3" />}
                                        placeholder={t("set_email", "Set email")}
                                        type="email"
                                        inputPlaceholder={t(
                                            "email_placeholder",
                                            "user@example.com",
                                        )}
                                    />
                                    <span className="mx-2">·</span>
                                    {/* 2. Phone */}
                                    <InlineEdit
                                        value={user.phone ?? ""}
                                        validate={optionalPhoneSchema}
                                        onSave={savePhone}
                                        icon={<Phone className="h-3 w-3" />}
                                        placeholder={t("set_phone", "Set phone")}
                                        type="tel"
                                        inputPlaceholder={t(
                                            "phone_placeholder",
                                            "+1 (555) 000-0000",
                                        )}
                                    />
                                    <span className="mx-2">·</span>
                                    <UserLocaleEdit />
                                    <UserTimezoneEdit />
                                    {/* 6. Created at */}
                                    <span>
                                        Created {formatDate(preferences, user.created_at, "PP")}
                                    </span>
                                </p>
                            </div>
                        </div>

                        <div className="shrink-0">
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
                    </div>

                    {/* Navigation Tabs */}
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
                        <DialogTitle>{t("delete_user")}</DialogTitle>
                        <DialogDescription>
                            {t(
                                "delete_user_warning",
                                "Are you sure you want to delete this user? This action cannot be undone.",
                            )}
                        </DialogDescription>
                    </DialogHeader>
                    <div className="py-4">
                        <div className="flex items-center gap-3 p-3 rounded-lg bg-muted">
                            <div
                                className="flex h-10 w-10 items-center justify-center rounded-lg shrink-0 text-white text-sm font-medium"
                                style={{ backgroundColor: userColor }}
                            >
                                {initials}
                            </div>
                            <div>
                                <p className="font-medium">{displayName}</p>
                                <p className="text-sm text-muted-foreground">
                                    {user.email ||
                                        getPrimaryExternalId(
                                            user as unknown as Record<string, unknown>,
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
                        <Button variant="destructive" onClick={deleteUser} disabled={isDeleting}>
                            {isDeleting ? t("deleting") : t("delete_user")}
                        </Button>
                    </DialogFooter>
                </DialogContent>
            </Dialog>
        </div>
    )
}
