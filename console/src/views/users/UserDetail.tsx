import { useContext, useState } from "react"
import { Outlet, useNavigate, NavLink, useLocation, Link } from "react-router"
import { useTranslation } from "react-i18next"
import { toast } from "react-hot-toast/headless"
import {
    Trash2,
    FileText,
    Activity,
    Bell,
    Route,
    Building2,
    ChevronRight,
    MoreHorizontal,
    Globe,
    Mail,
    Phone,
    Languages,
    Pencil,
    Check,
} from "lucide-react"
import { ProjectContext, UserContext } from "../../contexts"
import { PreferencesContext } from "../../ui/PreferencesContext"
import { getRandomColor } from "@/lib/colors"
import { formatDate, cn } from "../../utils"
import api from "../../api"
import {
    getTimezoneCoordinates,
    getLocalTimeInTimezone,
    getTimezoneOffset,
} from "@/lib/timezone-coordinates"

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
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover"
import {
    Command,
    CommandEmpty,
    CommandGroup,
    CommandInput,
    CommandItem,
    CommandList,
} from "@/components/ui/command"
import { Input } from "@/components/ui/input"
import type { User } from "../../types"

// eslint-disable-next-line @typescript-eslint/no-namespace
declare namespace Intl {
    type Key = "calendar" | "collation" | "currency" | "numberingSystem" | "timeZone" | "unit"
    function supportedValuesOf(input: Key): string[]
    class DisplayNames {
        constructor(locales: string[], options: { type: string })
        of(code: string): string | undefined
    }
}

export default function UserDetail() {
    const { t } = useTranslation()
    const navigate = useNavigate()
    const location = useLocation()
    const [preferences] = useContext(PreferencesContext)
    const [project] = useContext(ProjectContext)
    const [user, setUser] = useContext(UserContext)
    const [isDeleteOpen, setIsDeleteOpen] = useState(false)
    const [isDeleting, setIsDeleting] = useState(false)
    const [isTimezoneOpen, setIsTimezoneOpen] = useState(false)
    const [isSavingTimezone, setIsSavingTimezone] = useState(false)
    const [isEmailOpen, setIsEmailOpen] = useState(false)
    const [isSavingEmail, setIsSavingEmail] = useState(false)
    const [emailValue, setEmailValue] = useState(user.email ?? "")
    const [isPhoneOpen, setIsPhoneOpen] = useState(false)
    const [isSavingPhone, setIsSavingPhone] = useState(false)
    const [phoneValue, setPhoneValue] = useState(user.phone ?? "")
    const [isLocaleOpen, setIsLocaleOpen] = useState(false)
    const [isSavingLocale, setIsSavingLocale] = useState(false)

    const timeZones = Intl.supportedValuesOf("timeZone")

    const handleTimezoneChange = async (newTimezone: string) => {
        setIsSavingTimezone(true)
        try {
            const updatedUser = await api.users.update(project.id, user.id, {
                timezone: newTimezone,
            } as User)
            if (updatedUser) {
                setUser(updatedUser)
                toast.success(t("timezone_updated", "Timezone updated"))
            }
            setIsTimezoneOpen(false)
        } catch {
            toast.error(t("timezone_update_error", "Failed to update timezone"))
        } finally {
            setIsSavingTimezone(false)
        }
    }

    const handleEmailChange = async () => {
        const trimmedEmail = emailValue.trim()
        if (trimmedEmail === (user.email ?? "")) {
            setIsEmailOpen(false)
            return
        }
        setIsSavingEmail(true)
        try {
            const updatedUser = await api.users.update(project.id, user.id, {
                email: trimmedEmail || undefined,
            } as User)
            if (updatedUser) {
                setUser(updatedUser)
                toast.success(t("email_updated", "Email updated"))
            }
            setIsEmailOpen(false)
        } catch {
            toast.error(t("email_update_error", "Failed to update email"))
        } finally {
            setIsSavingEmail(false)
        }
    }

    const handlePhoneChange = async () => {
        const trimmedPhone = phoneValue.trim()
        if (trimmedPhone === (user.phone ?? "")) {
            setIsPhoneOpen(false)
            return
        }
        setIsSavingPhone(true)
        try {
            const updatedUser = await api.users.update(project.id, user.id, {
                phone: trimmedPhone || undefined,
            } as User)
            if (updatedUser) {
                setUser(updatedUser)
                toast.success(t("phone_updated", "Phone updated"))
            }
            setIsPhoneOpen(false)
        } catch {
            toast.error(t("phone_update_error", "Failed to update phone"))
        } finally {
            setIsSavingPhone(false)
        }
    }

    const handleLocaleChange = async (newLocale: string) => {
        setIsSavingLocale(true)
        try {
            const updatedUser = await api.users.update(project.id, user.id, {
                locale: newLocale,
            } as User)
            if (updatedUser) {
                setUser(updatedUser)
                toast.success(t("locale_updated", "Locale updated"))
            }
            setIsLocaleOpen(false)
        } catch {
            toast.error(t("locale_update_error", "Failed to update locale"))
        } finally {
            setIsSavingLocale(false)
        }
    }

    const userColor = getRandomColor(user.email ?? user.external_id ?? user.id)

    const displayName =
        user.full_name ??
        ((user.data as Record<string, unknown>)?.full_name as string) ??
        user.email ??
        "No name"

    const initials = (() => {
        const parts = displayName.split(/[\s@.]+/)
        if (parts.length >= 2) {
            return (parts[0][0] + parts[1][0]).toUpperCase()
        }
        return displayName.substring(0, 2).toUpperCase()
    })()

    // Determine active tab
    const basePath = `/projects/${project.id}/users/${user.id}`
    const currentPath = location.pathname
    const activeTab = currentPath === basePath ? "details" : currentPath.split("/").pop()

    const deleteUser = async () => {
        setIsDeleting(true)
        try {
            await api.users.delete(project.id, user.id)
            await navigate(`/projects/${project.id}/users`)
        } finally {
            setIsDeleting(false)
        }
    }

    const tabs = [
        { key: "details", to: "", label: t("details"), icon: FileText },
        { key: "events", to: "events", label: t("events"), icon: Activity },
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
                                className="ambient-map absolute inset-y-0 left-[30%] right-0 hidden lg:block pointer-events-none overflow-hidden opacity-[0.45] dark:opacity-[0.35]"
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

                <div className="p-6 pb-0 relative z-20">
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
                                    {user.email ? (
                                        <>
                                            <Popover
                                                open={isEmailOpen}
                                                onOpenChange={(open) => {
                                                    setIsEmailOpen(open)
                                                    if (open) setEmailValue(user.email ?? "")
                                                }}
                                            >
                                                <PopoverTrigger asChild>
                                                    <button
                                                        type="button"
                                                        className="inline-flex items-center gap-1 rounded-md px-1.5 py-0.5 -mx-1.5 -my-0.5 hover:bg-muted transition-colors group"
                                                    >
                                                        <Mail className="h-3 w-3" />
                                                        <span>{user.email}</span>
                                                        <Pencil className="h-2.5 w-2.5 opacity-0 group-hover:opacity-60 transition-opacity" />
                                                    </button>
                                                </PopoverTrigger>
                                                <PopoverContent className="w-72 p-3" align="start">
                                                    <form
                                                        onSubmit={(e) => {
                                                            e.preventDefault()
                                                            handleEmailChange()
                                                        }}
                                                        className="flex flex-col gap-2"
                                                    >
                                                        <Input
                                                            type="email"
                                                            value={emailValue}
                                                            onChange={(e) =>
                                                                setEmailValue(e.target.value)
                                                            }
                                                            placeholder={t(
                                                                "email_placeholder",
                                                                "user@example.com",
                                                            )}
                                                            autoFocus
                                                            disabled={isSavingEmail}
                                                        />
                                                        <div className="flex justify-end gap-2">
                                                            <Button
                                                                type="button"
                                                                variant="ghost"
                                                                size="sm"
                                                                onClick={() =>
                                                                    setIsEmailOpen(false)
                                                                }
                                                                disabled={isSavingEmail}
                                                            >
                                                                {t("cancel")}
                                                            </Button>
                                                            <Button
                                                                type="submit"
                                                                size="sm"
                                                                disabled={isSavingEmail}
                                                            >
                                                                {isSavingEmail
                                                                    ? t("saving", "Saving...")
                                                                    : t("save")}
                                                            </Button>
                                                        </div>
                                                    </form>
                                                </PopoverContent>
                                            </Popover>
                                            <span className="mx-2">·</span>
                                        </>
                                    ) : (
                                        <>
                                            <Popover
                                                open={isEmailOpen}
                                                onOpenChange={(open) => {
                                                    setIsEmailOpen(open)
                                                    if (open) setEmailValue("")
                                                }}
                                            >
                                                <PopoverTrigger asChild>
                                                    <button
                                                        type="button"
                                                        className="inline-flex items-center gap-1 rounded-md px-1.5 py-0.5 -mx-1.5 -my-0.5 hover:bg-muted transition-colors text-muted-foreground/60 hover:text-muted-foreground group"
                                                    >
                                                        <Mail className="h-3 w-3" />
                                                        {t("set_email", "Set email")}
                                                        <Pencil className="h-2.5 w-2.5 opacity-0 group-hover:opacity-60 transition-opacity" />
                                                    </button>
                                                </PopoverTrigger>
                                                <PopoverContent className="w-72 p-3" align="start">
                                                    <form
                                                        onSubmit={(e) => {
                                                            e.preventDefault()
                                                            handleEmailChange()
                                                        }}
                                                        className="flex flex-col gap-2"
                                                    >
                                                        <Input
                                                            type="email"
                                                            value={emailValue}
                                                            onChange={(e) =>
                                                                setEmailValue(e.target.value)
                                                            }
                                                            placeholder={t(
                                                                "email_placeholder",
                                                                "user@example.com",
                                                            )}
                                                            autoFocus
                                                            disabled={isSavingEmail}
                                                        />
                                                        <div className="flex justify-end gap-2">
                                                            <Button
                                                                type="button"
                                                                variant="ghost"
                                                                size="sm"
                                                                onClick={() =>
                                                                    setIsEmailOpen(false)
                                                                }
                                                                disabled={isSavingEmail}
                                                            >
                                                                {t("cancel")}
                                                            </Button>
                                                            <Button
                                                                type="submit"
                                                                size="sm"
                                                                disabled={isSavingEmail}
                                                            >
                                                                {isSavingEmail
                                                                    ? t("saving", "Saving...")
                                                                    : t("save")}
                                                            </Button>
                                                        </div>
                                                    </form>
                                                </PopoverContent>
                                            </Popover>
                                            <span className="mx-2">·</span>
                                        </>
                                    )}
                                    {/* 2. Phone */}
                                    {user.phone ? (
                                        <>
                                            <Popover
                                                open={isPhoneOpen}
                                                onOpenChange={(open) => {
                                                    setIsPhoneOpen(open)
                                                    if (open) setPhoneValue(user.phone ?? "")
                                                }}
                                            >
                                                <PopoverTrigger asChild>
                                                    <button
                                                        type="button"
                                                        className="inline-flex items-center gap-1 rounded-md px-1.5 py-0.5 -mx-1.5 -my-0.5 hover:bg-muted transition-colors group"
                                                    >
                                                        <Phone className="h-3 w-3" />
                                                        <span>{user.phone}</span>
                                                        <Pencil className="h-2.5 w-2.5 opacity-0 group-hover:opacity-60 transition-opacity" />
                                                    </button>
                                                </PopoverTrigger>
                                                <PopoverContent className="w-72 p-3" align="start">
                                                    <form
                                                        onSubmit={(e) => {
                                                            e.preventDefault()
                                                            handlePhoneChange()
                                                        }}
                                                        className="flex flex-col gap-2"
                                                    >
                                                        <Input
                                                            type="tel"
                                                            value={phoneValue}
                                                            onChange={(e) =>
                                                                setPhoneValue(e.target.value)
                                                            }
                                                            placeholder={t(
                                                                "phone_placeholder",
                                                                "+1 (555) 000-0000",
                                                            )}
                                                            autoFocus
                                                            disabled={isSavingPhone}
                                                        />
                                                        <div className="flex justify-end gap-2">
                                                            <Button
                                                                type="button"
                                                                variant="ghost"
                                                                size="sm"
                                                                onClick={() =>
                                                                    setIsPhoneOpen(false)
                                                                }
                                                                disabled={isSavingPhone}
                                                            >
                                                                {t("cancel")}
                                                            </Button>
                                                            <Button
                                                                type="submit"
                                                                size="sm"
                                                                disabled={isSavingPhone}
                                                            >
                                                                {isSavingPhone
                                                                    ? t("saving", "Saving...")
                                                                    : t("save")}
                                                            </Button>
                                                        </div>
                                                    </form>
                                                </PopoverContent>
                                            </Popover>
                                            <span className="mx-2">·</span>
                                        </>
                                    ) : (
                                        <>
                                            <Popover
                                                open={isPhoneOpen}
                                                onOpenChange={(open) => {
                                                    setIsPhoneOpen(open)
                                                    if (open) setPhoneValue("")
                                                }}
                                            >
                                                <PopoverTrigger asChild>
                                                    <button
                                                        type="button"
                                                        className="inline-flex items-center gap-1 rounded-md px-1.5 py-0.5 -mx-1.5 -my-0.5 hover:bg-muted transition-colors text-muted-foreground/60 hover:text-muted-foreground group"
                                                    >
                                                        <Phone className="h-3 w-3" />
                                                        {t("set_phone", "Set phone")}
                                                        <Pencil className="h-2.5 w-2.5 opacity-0 group-hover:opacity-60 transition-opacity" />
                                                    </button>
                                                </PopoverTrigger>
                                                <PopoverContent className="w-72 p-3" align="start">
                                                    <form
                                                        onSubmit={(e) => {
                                                            e.preventDefault()
                                                            handlePhoneChange()
                                                        }}
                                                        className="flex flex-col gap-2"
                                                    >
                                                        <Input
                                                            type="tel"
                                                            value={phoneValue}
                                                            onChange={(e) =>
                                                                setPhoneValue(e.target.value)
                                                            }
                                                            placeholder={t(
                                                                "phone_placeholder",
                                                                "+1 (555) 000-0000",
                                                            )}
                                                            autoFocus
                                                            disabled={isSavingPhone}
                                                        />
                                                        <div className="flex justify-end gap-2">
                                                            <Button
                                                                type="button"
                                                                variant="ghost"
                                                                size="sm"
                                                                onClick={() =>
                                                                    setIsPhoneOpen(false)
                                                                }
                                                                disabled={isSavingPhone}
                                                            >
                                                                {t("cancel")}
                                                            </Button>
                                                            <Button
                                                                type="submit"
                                                                size="sm"
                                                                disabled={isSavingPhone}
                                                            >
                                                                {isSavingPhone
                                                                    ? t("saving", "Saving...")
                                                                    : t("save")}
                                                            </Button>
                                                        </div>
                                                    </form>
                                                </PopoverContent>
                                            </Popover>
                                            <span className="mx-2">·</span>
                                        </>
                                    )}
                                    {/* 3. Locale */}
                                    {user.locale ? (
                                        <>
                                            <Popover
                                                open={isLocaleOpen}
                                                onOpenChange={setIsLocaleOpen}
                                            >
                                                <PopoverTrigger asChild>
                                                    <button
                                                        type="button"
                                                        className="inline-flex items-center gap-1 rounded-md px-1.5 py-0.5 -mx-1.5 -my-0.5 hover:bg-muted transition-colors group"
                                                    >
                                                        <Languages className="h-3 w-3" />
                                                        <span>{user.locale}</span>
                                                        <Pencil className="h-2.5 w-2.5 opacity-0 group-hover:opacity-60 transition-opacity" />
                                                    </button>
                                                </PopoverTrigger>
                                                <PopoverContent className="w-72 p-0" align="start">
                                                    <Command>
                                                        <CommandInput
                                                            placeholder={t(
                                                                "search_locale",
                                                                "Search locale...",
                                                            )}
                                                        />
                                                        <CommandList>
                                                            <CommandEmpty>
                                                                {t(
                                                                    "no_locale_found",
                                                                    "No locale found.",
                                                                )}
                                                            </CommandEmpty>
                                                            <CommandGroup>
                                                                {(() => {
                                                                    const locales = [
                                                                        ...new Set([
                                                                            ...navigator.languages,
                                                                            "en",
                                                                            "en-US",
                                                                            "en-GB",
                                                                            "es",
                                                                            "es-ES",
                                                                            "es-MX",
                                                                            "fr",
                                                                            "fr-FR",
                                                                            "de",
                                                                            "de-DE",
                                                                            "it",
                                                                            "it-IT",
                                                                            "pt",
                                                                            "pt-BR",
                                                                            "pt-PT",
                                                                            "nl",
                                                                            "nl-NL",
                                                                            "ja",
                                                                            "ja-JP",
                                                                            "ko",
                                                                            "ko-KR",
                                                                            "zh",
                                                                            "zh-CN",
                                                                            "zh-TW",
                                                                            "ar",
                                                                            "ar-SA",
                                                                            "hi",
                                                                            "hi-IN",
                                                                            "ru",
                                                                            "ru-RU",
                                                                            "pl",
                                                                            "pl-PL",
                                                                            "tr",
                                                                            "tr-TR",
                                                                            "sv",
                                                                            "sv-SE",
                                                                            "da",
                                                                            "da-DK",
                                                                            "no",
                                                                            "nb-NO",
                                                                            "fi",
                                                                            "fi-FI",
                                                                            "th",
                                                                            "th-TH",
                                                                            "vi",
                                                                            "vi-VN",
                                                                            "id",
                                                                            "id-ID",
                                                                            "ms",
                                                                            "ms-MY",
                                                                            "uk",
                                                                            "uk-UA",
                                                                            "cs",
                                                                            "cs-CZ",
                                                                            "ro",
                                                                            "ro-RO",
                                                                            "hu",
                                                                            "hu-HU",
                                                                            "el",
                                                                            "el-GR",
                                                                            "he",
                                                                            "he-IL",
                                                                            "bg",
                                                                            "bg-BG",
                                                                            "hr",
                                                                            "hr-HR",
                                                                            "sk",
                                                                            "sk-SK",
                                                                            "sl",
                                                                            "sl-SI",
                                                                            "lt",
                                                                            "lt-LT",
                                                                            "lv",
                                                                            "lv-LV",
                                                                            "et",
                                                                            "et-EE",
                                                                            "ca",
                                                                            "ca-ES",
                                                                            "fil",
                                                                            "fil-PH",
                                                                            "sw",
                                                                            "sw-KE",
                                                                            "af",
                                                                            "af-ZA",
                                                                        ]),
                                                                    ].sort()
                                                                    return locales.map((loc) => {
                                                                        let label = loc
                                                                        try {
                                                                            const dn =
                                                                                new Intl.DisplayNames(
                                                                                    ["en"],
                                                                                    {
                                                                                        type: "language",
                                                                                    },
                                                                                )
                                                                            label =
                                                                                dn.of(loc) ?? loc
                                                                        } catch {
                                                                            // keep raw code
                                                                        }
                                                                        return (
                                                                            <CommandItem
                                                                                key={loc}
                                                                                value={`${loc} ${label}`}
                                                                                onSelect={() =>
                                                                                    handleLocaleChange(
                                                                                        loc,
                                                                                    )
                                                                                }
                                                                                disabled={
                                                                                    isSavingLocale
                                                                                }
                                                                            >
                                                                                <Check
                                                                                    className={cn(
                                                                                        "mr-2 h-4 w-4",
                                                                                        user.locale ===
                                                                                            loc
                                                                                            ? "opacity-100"
                                                                                            : "opacity-0",
                                                                                    )}
                                                                                />
                                                                                <span>{label}</span>
                                                                                {label !== loc && (
                                                                                    <span className="ml-1 text-muted-foreground">
                                                                                        {loc}
                                                                                    </span>
                                                                                )}
                                                                            </CommandItem>
                                                                        )
                                                                    })
                                                                })()}
                                                            </CommandGroup>
                                                        </CommandList>
                                                    </Command>
                                                </PopoverContent>
                                            </Popover>
                                            <span className="mx-2">·</span>
                                        </>
                                    ) : (
                                        <>
                                            <Popover
                                                open={isLocaleOpen}
                                                onOpenChange={setIsLocaleOpen}
                                            >
                                                <PopoverTrigger asChild>
                                                    <button
                                                        type="button"
                                                        className="inline-flex items-center gap-1 rounded-md px-1.5 py-0.5 -mx-1.5 -my-0.5 hover:bg-muted transition-colors text-muted-foreground/60 hover:text-muted-foreground group"
                                                    >
                                                        <Languages className="h-3 w-3" />
                                                        {t("set_locale", "Set locale")}
                                                        <Pencil className="h-2.5 w-2.5 opacity-0 group-hover:opacity-60 transition-opacity" />
                                                    </button>
                                                </PopoverTrigger>
                                                <PopoverContent className="w-72 p-0" align="start">
                                                    <Command>
                                                        <CommandInput
                                                            placeholder={t(
                                                                "search_locale",
                                                                "Search locale...",
                                                            )}
                                                        />
                                                        <CommandList>
                                                            <CommandEmpty>
                                                                {t(
                                                                    "no_locale_found",
                                                                    "No locale found.",
                                                                )}
                                                            </CommandEmpty>
                                                            <CommandGroup>
                                                                {(() => {
                                                                    const locales = [
                                                                        ...new Set([
                                                                            ...navigator.languages,
                                                                            "en",
                                                                            "en-US",
                                                                            "en-GB",
                                                                            "es",
                                                                            "es-ES",
                                                                            "es-MX",
                                                                            "fr",
                                                                            "fr-FR",
                                                                            "de",
                                                                            "de-DE",
                                                                            "it",
                                                                            "it-IT",
                                                                            "pt",
                                                                            "pt-BR",
                                                                            "pt-PT",
                                                                            "nl",
                                                                            "nl-NL",
                                                                            "ja",
                                                                            "ja-JP",
                                                                            "ko",
                                                                            "ko-KR",
                                                                            "zh",
                                                                            "zh-CN",
                                                                            "zh-TW",
                                                                            "ar",
                                                                            "ar-SA",
                                                                            "hi",
                                                                            "hi-IN",
                                                                            "ru",
                                                                            "ru-RU",
                                                                            "pl",
                                                                            "pl-PL",
                                                                            "tr",
                                                                            "tr-TR",
                                                                            "sv",
                                                                            "sv-SE",
                                                                            "da",
                                                                            "da-DK",
                                                                            "no",
                                                                            "nb-NO",
                                                                            "fi",
                                                                            "fi-FI",
                                                                            "th",
                                                                            "th-TH",
                                                                            "vi",
                                                                            "vi-VN",
                                                                            "id",
                                                                            "id-ID",
                                                                            "ms",
                                                                            "ms-MY",
                                                                            "uk",
                                                                            "uk-UA",
                                                                            "cs",
                                                                            "cs-CZ",
                                                                            "ro",
                                                                            "ro-RO",
                                                                            "hu",
                                                                            "hu-HU",
                                                                            "el",
                                                                            "el-GR",
                                                                            "he",
                                                                            "he-IL",
                                                                            "bg",
                                                                            "bg-BG",
                                                                            "hr",
                                                                            "hr-HR",
                                                                            "sk",
                                                                            "sk-SK",
                                                                            "sl",
                                                                            "sl-SI",
                                                                            "lt",
                                                                            "lt-LT",
                                                                            "lv",
                                                                            "lv-LV",
                                                                            "et",
                                                                            "et-EE",
                                                                            "ca",
                                                                            "ca-ES",
                                                                            "fil",
                                                                            "fil-PH",
                                                                            "sw",
                                                                            "sw-KE",
                                                                            "af",
                                                                            "af-ZA",
                                                                        ]),
                                                                    ].sort()
                                                                    return locales.map((loc) => {
                                                                        let label = loc
                                                                        try {
                                                                            const dn =
                                                                                new Intl.DisplayNames(
                                                                                    ["en"],
                                                                                    {
                                                                                        type: "language",
                                                                                    },
                                                                                )
                                                                            label =
                                                                                dn.of(loc) ?? loc
                                                                        } catch {
                                                                            // keep raw code
                                                                        }
                                                                        return (
                                                                            <CommandItem
                                                                                key={loc}
                                                                                value={`${loc} ${label}`}
                                                                                onSelect={() =>
                                                                                    handleLocaleChange(
                                                                                        loc,
                                                                                    )
                                                                                }
                                                                                disabled={
                                                                                    isSavingLocale
                                                                                }
                                                                            >
                                                                                <span>{label}</span>
                                                                                {label !== loc && (
                                                                                    <span className="ml-1 text-muted-foreground">
                                                                                        {loc}
                                                                                    </span>
                                                                                )}
                                                                            </CommandItem>
                                                                        )
                                                                    })
                                                                })()}
                                                            </CommandGroup>
                                                        </CommandList>
                                                    </Command>
                                                </PopoverContent>
                                            </Popover>
                                            <span className="mx-2">·</span>
                                        </>
                                    )}
                                    {/* 4. External ID */}
                                    {user.external_id && (
                                        <>
                                            <code className="text-xs bg-muted px-1.5 py-0.5 rounded">
                                                {user.external_id}
                                            </code>
                                            <span className="mx-2">·</span>
                                        </>
                                    )}
                                    {/* 5. Timezone */}
                                    {user.timezone
                                        ? (() => {
                                              const localTime = getLocalTimeInTimezone(
                                                  user.timezone,
                                              )
                                              const utcOffset = getTimezoneOffset(user.timezone)
                                              return (
                                                  <>
                                                      <Popover
                                                          open={isTimezoneOpen}
                                                          onOpenChange={setIsTimezoneOpen}
                                                      >
                                                          <PopoverTrigger asChild>
                                                              <button
                                                                  type="button"
                                                                  className="inline-flex items-center gap-1 rounded-md px-1.5 py-0.5 -mx-1.5 -my-0.5 hover:bg-muted transition-colors group"
                                                              >
                                                                  <Globe className="h-3 w-3" />
                                                                  {localTime}
                                                                  <span className="text-muted-foreground/60">
                                                                      {utcOffset &&
                                                                          `(${utcOffset})`}
                                                                  </span>
                                                                  <Pencil className="h-2.5 w-2.5 opacity-0 group-hover:opacity-60 transition-opacity" />
                                                              </button>
                                                          </PopoverTrigger>
                                                          <PopoverContent
                                                              className="w-72 p-0"
                                                              align="start"
                                                          >
                                                              <Command>
                                                                  <CommandInput
                                                                      placeholder={t(
                                                                          "search_timezone",
                                                                          "Search timezone...",
                                                                      )}
                                                                  />
                                                                  <CommandList>
                                                                      <CommandEmpty>
                                                                          {t(
                                                                              "no_timezone_found",
                                                                              "No timezone found.",
                                                                          )}
                                                                      </CommandEmpty>
                                                                      <CommandGroup>
                                                                          {timeZones.map((tz) => (
                                                                              <CommandItem
                                                                                  key={tz}
                                                                                  value={tz}
                                                                                  onSelect={() =>
                                                                                      handleTimezoneChange(
                                                                                          tz,
                                                                                      )
                                                                                  }
                                                                                  disabled={
                                                                                      isSavingTimezone
                                                                                  }
                                                                              >
                                                                                  <Check
                                                                                      className={cn(
                                                                                          "mr-2 h-4 w-4",
                                                                                          user.timezone ===
                                                                                              tz
                                                                                              ? "opacity-100"
                                                                                              : "opacity-0",
                                                                                      )}
                                                                                  />
                                                                                  {tz}
                                                                              </CommandItem>
                                                                          ))}
                                                                      </CommandGroup>
                                                                  </CommandList>
                                                              </Command>
                                                          </PopoverContent>
                                                      </Popover>
                                                      <span className="mx-2">·</span>
                                                  </>
                                              )
                                          })()
                                        : (() => (
                                              <>
                                                  <Popover
                                                      open={isTimezoneOpen}
                                                      onOpenChange={setIsTimezoneOpen}
                                                  >
                                                      <PopoverTrigger asChild>
                                                          <button
                                                              type="button"
                                                              className="inline-flex items-center gap-1 rounded-md px-1.5 py-0.5 -mx-1.5 -my-0.5 hover:bg-muted transition-colors text-muted-foreground/60 hover:text-muted-foreground group"
                                                          >
                                                              <Globe className="h-3 w-3" />
                                                              {t("set_timezone", "Set timezone")}
                                                              <Pencil className="h-2.5 w-2.5 opacity-0 group-hover:opacity-60 transition-opacity" />
                                                          </button>
                                                      </PopoverTrigger>
                                                      <PopoverContent
                                                          className="w-72 p-0"
                                                          align="start"
                                                      >
                                                          <Command>
                                                              <CommandInput
                                                                  placeholder={t(
                                                                      "search_timezone",
                                                                      "Search timezone...",
                                                                  )}
                                                              />
                                                              <CommandList>
                                                                  <CommandEmpty>
                                                                      {t(
                                                                          "no_timezone_found",
                                                                          "No timezone found.",
                                                                      )}
                                                                  </CommandEmpty>
                                                                  <CommandGroup>
                                                                      {timeZones.map((tz) => (
                                                                          <CommandItem
                                                                              key={tz}
                                                                              value={tz}
                                                                              onSelect={() =>
                                                                                  handleTimezoneChange(
                                                                                      tz,
                                                                                  )
                                                                              }
                                                                              disabled={
                                                                                  isSavingTimezone
                                                                              }
                                                                          >
                                                                              {tz}
                                                                          </CommandItem>
                                                                      ))}
                                                                  </CommandGroup>
                                                              </CommandList>
                                                          </Command>
                                                      </PopoverContent>
                                                  </Popover>
                                                  <span className="mx-2">·</span>
                                              </>
                                          ))()}
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
                    <nav className="flex gap-1 mt-6 -mb-px">
                        {tabs.map((tab) => {
                            const Icon = tab.icon
                            const isActive = activeTab === tab.key
                            return (
                                <NavLink
                                    key={tab.key}
                                    to={tab.to}
                                    end={tab.to === ""}
                                    className={cn(
                                        "flex items-center gap-2 px-4 py-2.5 text-sm font-medium rounded-t-lg border-b-2 transition-colors",
                                        isActive
                                            ? "border-primary text-foreground bg-background"
                                            : "border-transparent text-muted-foreground hover:text-foreground hover:bg-muted/50",
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
                                    {user.email || user.external_id}
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
