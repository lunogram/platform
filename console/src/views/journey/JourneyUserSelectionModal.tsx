import { useState, useMemo } from "react"
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
import { ScrollArea, ScrollBar } from "@/components/ui/scroll-area"
import { Search, UserCircle2, Play } from "lucide-react"
import { getRandomColor } from "@/lib/colors"
import type { User } from "@/types"

interface UserSelectionModalProps {
    users: User[]
    isOpen: boolean
    onClose: () => void
    onSelect: (user: User) => void
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

export function UserSelectionModal({ users, isOpen, onClose, onSelect }: UserSelectionModalProps) {
    const { t } = useTranslation()
    const [searchTerm, setSearchTerm] = useState("")

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
                <DialogHeader className="px-4 pt-4 pb-3 sm:px-6 sm:pt-6 sm:pb-4">
                    <DialogTitle>
                        {t("select_user", "Select User")}
                    </DialogTitle>
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
                                            onClick={() => onSelect(user)}
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
                                                {user.email ?? "—"}
                                            </TableCell>
                                            <TableCell className="hidden md:table-cell text-muted-foreground">
                                                {user.timezone ?? "—"}
                                            </TableCell>
                                            <TableCell className="text-right">
                                                <Button
                                                    variant="ghost"
                                                    size="sm"
                                                    className="opacity-0 group-hover:opacity-100 transition-opacity"
                                                    onClick={(e) => {
                                                        e.stopPropagation()
                                                        onSelect(user)
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
            </DialogContent>
        </Dialog>
    )
}
