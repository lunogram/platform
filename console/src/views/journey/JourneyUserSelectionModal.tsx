import { useState, useMemo, useEffect, useCallback } from "react"
import { useTranslation } from "react-i18next"
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogHeader,
    DialogTitle,
} from "@/components/ui/dialog"
import {
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from "@/components/ui/table"
import { Input } from "@/components/ui/input"
import { Button } from "@/components/ui/button"
import { CodeEditor } from "@/components/ui/code-editor"
import { Search, UserCircle2, Play, ArrowLeft } from "lucide-react"
import { getRandomColor } from "@/lib/colors"
import type { User } from "@/types"
import api from "@/api"
import type { UUID } from "@/types/common"

interface UserSelectionModalProps {
    users: User[]
    isOpen: boolean
    onClose: () => void
    onSelect: (user: User, data?: Record<string, unknown>) => void
    projectId: UUID
    eventName?: string
}

function getUserDisplayName(user: User): string {
    if (user.full_name) return user.full_name
    if ((user.data as Record<string, unknown>)?.full_name)
        return (user.data as Record<string, unknown>).full_name as string
    if (user.email) return user.email
    return user.external_id ?? "Unknown"
}

function getUserInitials(user: User): string {
    const name = getUserDisplayName(user)
    const parts = name.trim().split(/[\s@.]+/)
    if (parts.length >= 2) {
        return (parts[0][0] + parts[parts.length - 1][0]).toUpperCase()
    }
    return name.substring(0, 2).toUpperCase()
}

function getUserSubtext(user: User): string | null {
    if (user.full_name && user.email) return user.email
    if (user.full_name && user.external_id) return user.external_id
    if (user.email && user.external_id) return user.external_id
    return null
}

export function UserSelectionModal({
    users,
    isOpen,
    onClose,
    onSelect,
    projectId,
    eventName,
}: UserSelectionModalProps) {
    const { t } = useTranslation()
    const [searchTerm, setSearchTerm] = useState("")
    const [selectedUser, setSelectedUser] = useState<User | null>(null)
    const [schemaFields, setSchemaFields] = useState<{ path: string; types: string[] }[]>([])
    const [jsonValue, setJsonValue] = useState("")
    const [jsonError, setJsonError] = useState<string | undefined>(undefined)

    // Fetch the event schema for the entrance's event_name
    useEffect(() => {
        if (!isOpen || !eventName) {
            setSchemaFields([])
            return
        }

        let cancelled = false
        api.projects
            .pathSuggestions(projectId)
            .then((suggestions) => {
                if (cancelled) return
                const match = suggestions.eventPaths.find((e) => e.name === eventName)
                if (match?.schema) {
                    // Only include fields under .data.* (the user-defined event properties)
                    const dataFields = match.schema.filter((f) => f.path.startsWith(".data."))
                    setSchemaFields(dataFields)
                } else {
                    setSchemaFields([])
                }
            })
            .catch(console.error)

        return () => {
            cancelled = true
        }
    }, [isOpen, eventName, projectId])

    // Reset state when modal closes
    useEffect(() => {
        if (!isOpen) {
            setSelectedUser(null)
            setJsonValue("")
            setJsonError(undefined)
            setSearchTerm("")
        }
    }, [isOpen])

    const handleUserClick = useCallback(
        (user: User) => {
            if (schemaFields.length > 0) {
                // Build scaffold JSON from schema fields with empty values
                const scaffold: Record<string, string> = {}
                for (const field of schemaFields) {
                    const key = field.path.replace(/^\.data\./, "").replace(/^\./, "")
                    scaffold[key] = ""
                }
                setJsonValue(JSON.stringify(scaffold, null, 2))
                setJsonError(undefined)
                setSelectedUser(user)
            } else {
                // No schema — trigger immediately
                onSelect(user)
            }
        },
        [schemaFields, onSelect],
    )

    const handleFormSubmit = useCallback(() => {
        if (!selectedUser) return

        // Parse the JSON from the editor
        let data: Record<string, unknown> | undefined
        if (jsonValue.trim()) {
            try {
                const parsed = JSON.parse(jsonValue)
                if (typeof parsed === "object" && parsed !== null && !Array.isArray(parsed)) {
                    // Only pass data if there are non-empty values
                    const hasValues = Object.values(parsed).some(
                        (v) => v !== "" && v !== null && v !== undefined,
                    )
                    if (hasValues) data = parsed
                }
            } catch {
                // Should not happen — Run button is disabled when there's a JSON error
                return
            }
        }

        onSelect(selectedUser, data)
    }, [selectedUser, jsonValue, onSelect])

    const handleBack = useCallback(() => {
        setSelectedUser(null)
        setJsonValue("")
        setJsonError(undefined)
    }, [])

    const filteredUsers = useMemo(() => {
        if (!searchTerm.trim()) return users
        const term = searchTerm.toLowerCase()
        return users.filter((user) => {
            const searchable = [
                user.full_name,
                user.email,
                user.external_id,
                user.phone,
                user.timezone,
                ...Object.values(user.data).map(String),
            ]
                .filter(Boolean)
                .join(" ")
                .toLowerCase()
            return searchable.includes(term)
        })
    }, [users, searchTerm])

    return (
        <Dialog open={isOpen} onOpenChange={(open) => !open && onClose()}>
            <DialogContent className="sm:max-w-2xl max-h-[85vh] min-h-[400px] flex flex-col gap-0 p-0">
                {selectedUser ? (
                    <>
                        <DialogHeader className="px-4 pt-4 pb-3 sm:px-6 sm:pt-6 sm:pb-4">
                            <DialogTitle>{t("event_data", "Event Data")}</DialogTitle>
                            <DialogDescription>
                                {t(
                                    "event_data_description",
                                    "Provide event data for the journey entrance. These values will be available as journey variables.",
                                )}
                            </DialogDescription>
                        </DialogHeader>

                        <div className="flex-1 min-h-0 overflow-y-auto border-t px-4 py-4 sm:px-6 space-y-4">
                            <div className="flex items-center gap-3 rounded-md border p-3">
                                <div
                                    className="flex h-8 w-8 items-center justify-center rounded-full text-white text-xs font-medium shrink-0"
                                    style={{
                                        backgroundColor: getRandomColor(
                                            selectedUser.email ??
                                                selectedUser.external_id ??
                                                selectedUser.id,
                                        ),
                                    }}
                                >
                                    {getUserInitials(selectedUser)}
                                </div>
                                <div className="min-w-0">
                                    <div className="font-medium text-sm truncate">
                                        {getUserDisplayName(selectedUser)}
                                    </div>
                                    {getUserSubtext(selectedUser) && (
                                        <div className="text-xs text-muted-foreground truncate">
                                            {getUserSubtext(selectedUser)}
                                        </div>
                                    )}
                                </div>
                            </div>

                            <CodeEditor
                                language="json"
                                value={jsonValue}
                                onChange={setJsonValue}
                                onError={setJsonError}
                                requireObject
                                minHeight="120px"
                                maxHeight="300px"
                                placeholder='{ "key": "value" }'
                            />
                        </div>

                        <div className="border-t px-4 py-3 sm:px-6 flex justify-between">
                            <Button variant="outline" onClick={handleBack}>
                                <ArrowLeft className="h-4 w-4 mr-1.5" />
                                {t("back", "Back")}
                            </Button>
                            <Button onClick={handleFormSubmit} disabled={!!jsonError}>
                                <Play className="h-3.5 w-3.5 mr-1.5 fill-current" />
                                {t("run", "Run")}
                            </Button>
                        </div>
                    </>
                ) : (
                    <>
                        <DialogHeader className="px-4 pt-4 pb-3 sm:px-6 sm:pt-6 sm:pb-4">
                            <DialogTitle>{t("select_user", "Select User")}</DialogTitle>
                            <DialogDescription>
                                {t(
                                    "select_user_description",
                                    "Choose a user to initiate the journey.",
                                )}
                            </DialogDescription>
                            <div className="relative mt-3">
                                <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                                <Input
                                    placeholder={t("search_users", "Search users...")}
                                    value={searchTerm}
                                    onChange={(e) => setSearchTerm(e.target.value)}
                                    className="pl-9"
                                />
                            </div>
                        </DialogHeader>

                        <div className="flex-1 min-h-0 overflow-auto border-t">
                            <Table>
                                <TableHeader className="bg-background sticky top-0 z-10">
                                    <TableRow>
                                        <TableHead>{t("user", "User")}</TableHead>
                                        <TableHead className="hidden sm:table-cell">
                                            {t("email", "Email")}
                                        </TableHead>
                                        <TableHead className="hidden md:table-cell">
                                            {t("timezone", "Timezone")}
                                        </TableHead>
                                        <TableHead className="w-[1%]" />
                                    </TableRow>
                                </TableHeader>
                                <TableBody>
                                    {filteredUsers.length > 0 ? (
                                        filteredUsers.map((user) => {
                                            const color = getRandomColor(
                                                user.email ?? user.external_id ?? user.id,
                                            )
                                            const subtext = getUserSubtext(user)
                                            return (
                                                <TableRow
                                                    key={user.id}
                                                    className="group cursor-pointer"
                                                    onClick={() => handleUserClick(user)}
                                                >
                                                    <TableCell>
                                                        <div className="flex items-center gap-3 py-0.5">
                                                            <div
                                                                className="flex h-8 w-8 items-center justify-center rounded-full text-white text-xs font-medium shrink-0"
                                                                style={{ backgroundColor: color }}
                                                            >
                                                                {getUserInitials(user)}
                                                            </div>
                                                            <div className="min-w-0">
                                                                <div className="font-medium text-sm truncate">
                                                                    {getUserDisplayName(user)}
                                                                </div>
                                                                {subtext && (
                                                                    <div className="text-xs text-muted-foreground truncate">
                                                                        {subtext}
                                                                    </div>
                                                                )}
                                                            </div>
                                                        </div>
                                                    </TableCell>
                                                    <TableCell className="hidden sm:table-cell text-muted-foreground">
                                                        {user.email ?? "---"}
                                                    </TableCell>
                                                    <TableCell className="hidden md:table-cell text-muted-foreground">
                                                        {user.timezone ?? "---"}
                                                    </TableCell>
                                                    <TableCell className="text-right">
                                                        <Button
                                                            variant="ghost"
                                                            size="sm"
                                                            className="opacity-0 group-hover:opacity-100 transition-opacity"
                                                            onClick={(e) => {
                                                                e.stopPropagation()
                                                                handleUserClick(user)
                                                            }}
                                                        >
                                                            <Play className="h-3.5 w-3.5 mr-1.5 fill-current" />
                                                            {t("run", "Run")}
                                                        </Button>
                                                    </TableCell>
                                                </TableRow>
                                            )
                                        })
                                    ) : (
                                        <TableRow>
                                            <TableCell colSpan={4} className="h-32 text-center">
                                                <div className="flex flex-col items-center gap-2 text-muted-foreground">
                                                    <UserCircle2 className="h-8 w-8" />
                                                    <p>
                                                        {searchTerm
                                                            ? t("no_users_found", "No users found")
                                                            : t("no_users_yet", "No users yet")}
                                                    </p>
                                                </div>
                                            </TableCell>
                                        </TableRow>
                                    )}
                                </TableBody>
                            </Table>
                        </div>

                        {filteredUsers.length > 0 && (
                            <div className="border-t px-4 py-3 sm:px-6">
                                <p className="text-sm text-muted-foreground">
                                    {filteredUsers.length}{" "}
                                    {filteredUsers.length === 1
                                        ? t("user", "user")
                                        : t("users", "users")}
                                </p>
                            </div>
                        )}
                    </>
                )}
            </DialogContent>
        </Dialog>
    )
}
