import { useCallback, useContext, useState } from "react"
import { useTranslation } from "react-i18next"
import { Search, User, Mail, Hash } from "lucide-react"
import type { User as UserType } from "../../types"
import { getUserDisplayName, getUserInitials } from "@/lib/name"
import { ProjectContext } from "../../contexts"
import api from "../../api"
import { useResolver } from "../../hooks"

import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
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
import { Skeleton } from "@/components/ui/skeleton"

interface UserLookupProps {
    open: boolean
    onClose: (open: boolean) => void
    onSelected: (user: UserType) => Promise<void> | void
}

export const UserLookup = ({ open, onClose, onSelected }: UserLookupProps) => {
    const [project] = useContext(ProjectContext)
    const { t } = useTranslation()
    const [searchQuery, setSearchQuery] = useState("")
    const [debouncedQuery, setDebouncedQuery] = useState("")
    const [isSearching, setIsSearching] = useState(false)
    const [isSelecting, setIsSelecting] = useState(false)

    const [users] = useResolver(
        useCallback(async () => {
            if (!open) return []
            setIsSearching(true)
            try {
                const result = await api.users.search(project.id, {
                    search: debouncedQuery || undefined,
                    limit: 20,
                })
                return result.results
            } finally {
                setIsSearching(false)
            }
        }, [project.id, debouncedQuery, open]),
    )

    const handleSearch = (e: React.FormEvent) => {
        e.preventDefault()
        setDebouncedQuery(searchQuery)
    }

    const handleSelect = async (user: UserType) => {
        setIsSelecting(true)
        try {
            await onSelected(user)
            onClose(false)
            setSearchQuery("")
            setDebouncedQuery("")
        } finally {
            setIsSelecting(false)
        }
    }

    const handleOpenChange = (isOpen: boolean) => {
        if (!isOpen) {
            setSearchQuery("")
            setDebouncedQuery("")
        }
        onClose(isOpen)
    }

    return (
        <Dialog open={open} onOpenChange={handleOpenChange}>
            <DialogContent className="sm:max-w-2xl max-h-[80vh] flex flex-col">
                <DialogHeader>
                    <DialogTitle>{t("user_lookup")}</DialogTitle>
                    <DialogDescription>
                        {t(
                            "user_lookup_description",
                            "Search for a user by email, name, or external ID",
                        )}
                    </DialogDescription>
                </DialogHeader>

                {/* Search Form */}
                <form onSubmit={handleSearch} className="flex gap-2">
                    <div className="relative flex-1">
                        <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                        <Input
                            placeholder={t("enter_email")}
                            value={searchQuery}
                            onChange={(e) => setSearchQuery(e.target.value)}
                            className="pl-9"
                            autoFocus
                        />
                    </div>
                    <Button type="submit" variant="secondary">
                        {t("search")}
                    </Button>
                </form>

                {/* Results */}
                <div className="flex-1 overflow-auto rounded-lg border bg-card">
                    <Table>
                        <TableHeader>
                            <TableRow>
                                <TableHead>{t("name")}</TableHead>
                                <TableHead>{t("email")}</TableHead>
                                <TableHead>{t("external_id")}</TableHead>
                            </TableRow>
                        </TableHeader>
                        <TableBody>
                            {isSearching || users == null ? (
                                // Loading skeleton
                                Array.from({ length: 3 }).map((_, i) => (
                                    <TableRow key={i}>
                                        <TableCell>
                                            <div className="flex items-center gap-3">
                                                <Skeleton className="h-8 w-8 rounded-full" />
                                                <Skeleton className="h-4 w-28" />
                                            </div>
                                        </TableCell>
                                        <TableCell>
                                            <Skeleton className="h-4 w-36" />
                                        </TableCell>
                                        <TableCell>
                                            <Skeleton className="h-4 w-20" />
                                        </TableCell>
                                    </TableRow>
                                ))
                            ) : users.length === 0 ? (
                                <TableRow>
                                    <TableCell colSpan={3} className="h-32 text-center">
                                        <div className="flex flex-col items-center gap-2 text-muted-foreground">
                                            <User className="h-8 w-8" />
                                            <p>{t("no_users_found")}</p>
                                        </div>
                                    </TableCell>
                                </TableRow>
                            ) : (
                                users?.map((user) => (
                                    <TableRow
                                        key={user.id}
                                        className="cursor-pointer hover:bg-muted/50"
                                        onClick={() => !isSelecting && handleSelect(user)}
                                    >
                                        <TableCell>
                                            <div className="flex items-center gap-3">
                                                <div className="flex h-8 w-8 items-center justify-center rounded-full bg-primary/10 text-primary text-xs font-medium">
                                                    {getUserInitials(user)}
                                                </div>
                                                <span className="font-medium">
                                                    {getUserDisplayName(user)}
                                                </span>
                                            </div>
                                        </TableCell>
                                        <TableCell>
                                            {user.email ? (
                                                <div className="flex items-center gap-2 text-muted-foreground">
                                                    <Mail className="h-3 w-3" />
                                                    <span className="text-sm">{user.email}</span>
                                                </div>
                                            ) : (
                                                <span className="text-sm text-muted-foreground">
                                                    —
                                                </span>
                                            )}
                                        </TableCell>
                                        <TableCell>
                                            {user.external_id ? (
                                                <div className="flex items-center gap-2 text-muted-foreground">
                                                    <Hash className="h-3 w-3" />
                                                    <code className="text-sm">
                                                        {user.external_id}
                                                    </code>
                                                </div>
                                            ) : (
                                                <span className="text-sm text-muted-foreground">
                                                    —
                                                </span>
                                            )}
                                        </TableCell>
                                    </TableRow>
                                ))
                            )}
                        </TableBody>
                    </Table>
                </div>
            </DialogContent>
        </Dialog>
    )
}
