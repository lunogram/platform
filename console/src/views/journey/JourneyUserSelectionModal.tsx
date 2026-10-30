import { useState, useEffect, useCallback, useMemo, useRef } from "react"
import { useTranslation } from "react-i18next"
import { useInfiniteQuery, useQuery } from "@tanstack/react-query"
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogHeader,
    DialogTitle,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Button } from "@/components/ui/button"
import { CodeEditor } from "@/components/ui/code-editor"
import { Loader2, Search, UserCircle2, Play, ArrowLeft } from "lucide-react"
import { UserCell } from "./components/UserCell"
import {
    getUserDisplayName,
    getUserInitials,
    getUserSubtext,
    getPrimaryExternalId,
} from "./components/userUtils"
import { getRandomColor } from "@/lib/colors"
import { useDebounceControl } from "@/hooks"
import type { User } from "@/types"
import type { UUID } from "@/types/common"
import { pathSuggestionsQuery, userPagesQuery } from "./queries"

interface UserSelectionModalProps {
    isOpen: boolean
    onClose: () => void
    onSelect: (user: User, data?: Record<string, unknown>) => void
    projectId: UUID
    eventName?: string
}

export function UserSelectionModal({
    isOpen,
    onClose,
    onSelect,
    projectId,
    eventName,
}: UserSelectionModalProps) {
    const { t } = useTranslation()
    const [selectedUser, setSelectedUser] = useState<User | null>(null)
    const [jsonValue, setJsonValue] = useState("")
    const [jsonError, setJsonError] = useState<string | undefined>(undefined)

    const [search, setSearch] = useState("")
    const scrollRef = useRef<HTMLDivElement>(null)

    // Debounced search — triggers server-side search
    const [searchInput, setSearchInput] = useDebounceControl(search, (value) => {
        setSearch(value)
    })

    // Pages are keyed by search, so a search typed while a page loads is
    // fetched too, and an older search's pages never show for a newer one.
    const userPages = useInfiniteQuery({
        ...userPagesQuery(projectId, search),
        enabled: isOpen,
    })
    const { hasNextPage, isFetchingNextPage, fetchNextPage } = userPages
    const users = useMemo(
        () => userPages.data?.pages.flatMap((page) => page.results) ?? [],
        [userPages.data],
    )
    const total = userPages.data?.pages.at(-1)?.total ?? 0
    const loading = userPages.isFetching

    // Reset state when modal closes
    useEffect(() => {
        if (!isOpen) {
            setSelectedUser(null)
            setJsonValue("")
            setJsonError(undefined)
            setSearchInput("")
            setSearch("")
        }
    }, [isOpen, setSearchInput])

    // Infinite scroll handler
    const handleScroll = useCallback(() => {
        const el = scrollRef.current
        if (!el || !hasNextPage || isFetchingNextPage) return

        const nearBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 100
        if (nearBottom) void fetchNextPage()
    }, [hasNextPage, isFetchingNextPage, fetchNextPage])

    // The event schema for the entrance's event_name
    const suggestions = useQuery({
        ...pathSuggestionsQuery(projectId),
        enabled: isOpen && !!eventName,
    })
    const schemaFields = useMemo(() => {
        if (!isOpen || !eventName) return []
        const match = suggestions.data?.eventPaths.find((e) => e.name === eventName)
        return match?.schema?.filter((f) => f.path.startsWith(".data.")) ?? []
    }, [isOpen, eventName, suggestions.data])

    const handleUserClick = useCallback(
        (user: User) => {
            if (schemaFields.length > 0) {
                const scaffold: Record<string, string> = {}
                for (const field of schemaFields) {
                    const key = field.path.replace(/^\.data\./, "").replace(/^\./, "")
                    scaffold[key] = ""
                }
                setJsonValue(JSON.stringify(scaffold, null, 2))
                setJsonError(undefined)
                setSelectedUser(user)
            } else {
                onSelect(user)
            }
        },
        [schemaFields, onSelect],
    )

    const handleFormSubmit = useCallback(() => {
        if (!selectedUser) return

        let data: Record<string, unknown> | undefined
        if (jsonValue.trim()) {
            try {
                const parsed = JSON.parse(jsonValue)
                if (typeof parsed === "object" && parsed !== null && !Array.isArray(parsed)) {
                    const hasValues = Object.values(parsed).some(
                        (v) => v !== "" && v !== null && v !== undefined,
                    )
                    if (hasValues) data = parsed
                }
            } catch {
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

    const hasMore = !!hasNextPage

    return (
        <Dialog open={isOpen} onOpenChange={(open) => !open && onClose()}>
            <DialogContent className="w-3/4 max-w-3xl max-h-[85vh] flex flex-col gap-0 p-0">
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
                                                getPrimaryExternalId(
                                                    selectedUser as unknown as Record<
                                                        string,
                                                        unknown
                                                    >,
                                                ) ??
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
                                    aria-label={t("search_users", "Search users")}
                                    value={searchInput}
                                    onChange={(e) => setSearchInput(e.target.value)}
                                    className="pl-9"
                                />
                            </div>
                        </DialogHeader>

                        <div
                            ref={scrollRef}
                            className="flex-1 min-h-0 overflow-auto border-t"
                            onScroll={handleScroll}
                        >
                            {users.length > 0 ? (
                                <div className="divide-y">
                                    {users.map((user) => (
                                        <button
                                            key={user.id}
                                            type="button"
                                            className="group flex w-full items-center justify-between px-4 sm:px-6 py-2 cursor-pointer hover:bg-muted/50 transition-colors text-left focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-inset"
                                            onClick={() => handleUserClick(user)}
                                        >
                                            <UserCell user={user} />
                                            <span
                                                className="inline-flex h-8 shrink-0 items-center justify-center gap-2 rounded-md px-3 text-xs font-medium opacity-0 transition-opacity group-hover:opacity-100"
                                                aria-hidden="true"
                                            >
                                                <Play className="h-3.5 w-3.5 mr-1.5 fill-current" />
                                                {t("run", "Run")}
                                            </span>
                                        </button>
                                    ))}
                                    {loading && hasMore && (
                                        <div className="flex justify-center py-4">
                                            <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
                                        </div>
                                    )}
                                </div>
                            ) : loading ? (
                                <div className="flex justify-center py-16">
                                    <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
                                </div>
                            ) : userPages.isError ? (
                                <div className="flex flex-col items-center gap-3 py-16 text-muted-foreground">
                                    <p>{t("journey_users_load_failed", "Couldn't load users.")}</p>
                                    <Button
                                        variant="outline"
                                        size="sm"
                                        onClick={() => userPages.refetch()}
                                    >
                                        {t("retry", "Retry")}
                                    </Button>
                                </div>
                            ) : (
                                <div className="flex flex-col items-center gap-2 py-16 text-muted-foreground">
                                    <UserCircle2 className="h-8 w-8" />
                                    <p>
                                        {searchInput
                                            ? t("no_users_found", "No users found")
                                            : t("no_users_yet", "No users yet")}
                                    </p>
                                </div>
                            )}
                        </div>

                        {users.length > 0 && (
                            <div className="border-t px-4 py-3 sm:px-6">
                                <p className="text-sm text-muted-foreground">
                                    {users.length} of {total}{" "}
                                    {total === 1 ? t("user", "user") : t("users", "users")}
                                </p>
                            </div>
                        )}
                    </>
                )}
            </DialogContent>
        </Dialog>
    )
}
