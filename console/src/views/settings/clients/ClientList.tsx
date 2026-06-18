import { useState } from "react"
import { useNavigate, useParams } from "react-router"
import { toast } from "sonner"
import {
    Plus,
    KeyRound,
    ShieldCheck,
    Clock,
    MoreHorizontal,
    Trash2,
    Globe,
    UserRound,
} from "lucide-react"
import { snakeToTitle } from "@/utils"
import { describeIdentity, permissionSummary, type IdentityType } from "./model"
import { removeClient, useClients } from "./store"

import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import {
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from "@/components/ui/table"
import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import {
    AlertDialog,
    AlertDialogAction,
    AlertDialogCancel,
    AlertDialogContent,
    AlertDialogDescription,
    AlertDialogFooter,
    AlertDialogHeader,
    AlertDialogTitle,
} from "@/components/ui/alert-dialog"

const identityIcon: Record<IdentityType, typeof KeyRound> = {
    api_key: KeyRound,
    trusted_issuer: ShieldCheck,
    session: Clock,
}

// ClientList is the index of the API & Clients section: a table of every client
// with its permissions, authentication, and data scope.
export default function ClientList() {
    const { projectId = "" } = useParams()
    const { clients, loading, reload } = useClients(projectId)
    const navigate = useNavigate()
    const [pendingDelete, setPendingDelete] = useState<{ id: string; name: string } | null>(null)

    const confirmDelete = async () => {
        if (!pendingDelete) return
        try {
            await removeClient(projectId, pendingDelete.id)
            reload()
        } catch {
            toast.error("Couldn't delete the client. Please try again.")
        } finally {
            setPendingDelete(null)
        }
    }

    return (
        <div className="flex flex-col gap-6">
            <div className="flex flex-col justify-between gap-3 sm:flex-row sm:items-center">
                <div className="flex flex-col gap-1">
                    <h2 className="text-2xl font-semibold tracking-tight">API & Clients</h2>
                    <p className="text-sm text-ink-soft">
                        How clients authenticate to your project, and what they may do.
                    </p>
                </div>
                <Button onClick={() => navigate("new")}>
                    <Plus className="mr-2 h-4 w-4" />
                    New client
                </Button>
            </div>

            <div className="overflow-hidden rounded-xl border bg-card">
                <Table>
                    <TableHeader>
                        <TableRow className="hover:bg-transparent">
                            <TableHead>Name</TableHead>
                            <TableHead>Permissions</TableHead>
                            <TableHead className="hidden md:table-cell">Authentication</TableHead>
                            <TableHead className="hidden lg:table-cell">Data</TableHead>
                            <TableHead className="w-[52px]" />
                        </TableRow>
                    </TableHeader>
                    <TableBody>
                        {loading ? (
                            <TableRow className="hover:bg-transparent">
                                <TableCell
                                    colSpan={5}
                                    className="h-36 text-center text-sm text-ink-soft"
                                >
                                    Loading…
                                </TableCell>
                            </TableRow>
                        ) : clients.length === 0 ? (
                            <TableRow className="hover:bg-transparent">
                                <TableCell colSpan={5} className="h-36 text-center">
                                    <div className="flex flex-col items-center gap-2 text-ink-soft">
                                        <KeyRound className="h-7 w-7" strokeWidth={1.5} />
                                        <p className="text-sm">No clients yet</p>
                                    </div>
                                </TableCell>
                            </TableRow>
                        ) : (
                            clients.map((client) => {
                                const Icon = identityIcon[client.identity.type]
                                return (
                                    <TableRow
                                        key={client.id}
                                        className="cursor-pointer hover:bg-surface-soft"
                                        onClick={() => navigate(client.id)}
                                    >
                                        <TableCell className="py-3">
                                            <div className="flex items-center gap-3">
                                                <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-surface-muted text-ink-soft">
                                                    <Icon
                                                        className="h-[18px] w-[18px]"
                                                        strokeWidth={1.5}
                                                    />
                                                </div>
                                                <div className="flex min-w-0 flex-col">
                                                    <span className="font-medium leading-tight">
                                                        {client.name}
                                                    </span>
                                                    {client.description && (
                                                        <span className="max-w-[19rem] truncate text-xs text-ink-soft">
                                                            {client.description}
                                                        </span>
                                                    )}
                                                </div>
                                            </div>
                                        </TableCell>
                                        <TableCell>
                                            <Badge variant="secondary" className="font-medium">
                                                {client.permissions.kind === "role"
                                                    ? snakeToTitle(client.permissions.role)
                                                    : permissionSummary(client.permissions)}
                                            </Badge>
                                        </TableCell>
                                        <TableCell className="hidden md:table-cell">
                                            <span className="inline-flex max-w-[13rem] items-center gap-1.5 truncate rounded-md bg-surface-muted px-2 py-1 font-mono text-xs text-ink-soft">
                                                <Icon
                                                    className="h-3.5 w-3.5 shrink-0"
                                                    strokeWidth={1.75}
                                                />
                                                <span className="truncate">
                                                    {describeIdentity(client.identity)}
                                                </span>
                                            </span>
                                        </TableCell>
                                        <TableCell className="hidden lg:table-cell">
                                            <span className="inline-flex items-center gap-1.5 whitespace-nowrap text-sm text-ink-soft">
                                                {client.subjectScope === "own" ? (
                                                    <>
                                                        <UserRound
                                                            className="h-3.5 w-3.5"
                                                            strokeWidth={1.75}
                                                        />
                                                        Own data
                                                    </>
                                                ) : (
                                                    <>
                                                        <Globe
                                                            className="h-3.5 w-3.5"
                                                            strokeWidth={1.75}
                                                        />
                                                        All data
                                                    </>
                                                )}
                                            </span>
                                        </TableCell>
                                        <TableCell onClick={(e) => e.stopPropagation()}>
                                            <DropdownMenu>
                                                <DropdownMenuTrigger asChild>
                                                    <Button
                                                        variant="ghost"
                                                        size="icon"
                                                        className="h-8 w-8 text-ink-soft"
                                                    >
                                                        <MoreHorizontal className="h-4 w-4" />
                                                    </Button>
                                                </DropdownMenuTrigger>
                                                <DropdownMenuContent align="end">
                                                    <DropdownMenuItem
                                                        className="text-destructive focus:text-destructive"
                                                        onClick={() =>
                                                            setPendingDelete({
                                                                id: client.id,
                                                                name: client.name,
                                                            })
                                                        }
                                                    >
                                                        <Trash2 className="mr-2 h-4 w-4" />
                                                        Delete
                                                    </DropdownMenuItem>
                                                </DropdownMenuContent>
                                            </DropdownMenu>
                                        </TableCell>
                                    </TableRow>
                                )
                            })
                        )}
                    </TableBody>
                </Table>
                {clients.length > 0 && (
                    <div className="border-t px-4 py-2.5 text-xs text-ink-soft">
                        {clients.length} {clients.length === 1 ? "client" : "clients"}
                    </div>
                )}
            </div>

            <AlertDialog
                open={pendingDelete !== null}
                onOpenChange={(open) => !open && setPendingDelete(null)}
            >
                <AlertDialogContent>
                    <AlertDialogHeader>
                        <AlertDialogTitle>Delete this client?</AlertDialogTitle>
                        <AlertDialogDescription>
                            {pendingDelete?.name
                                ? `"${pendingDelete.name}" will lose access immediately. This cannot be undone.`
                                : "This client will lose access immediately. This cannot be undone."}
                        </AlertDialogDescription>
                    </AlertDialogHeader>
                    <AlertDialogFooter>
                        <AlertDialogCancel>Cancel</AlertDialogCancel>
                        <AlertDialogAction
                            className="bg-destructive text-white hover:bg-destructive/90"
                            onClick={confirmDelete}
                        >
                            Delete client
                        </AlertDialogAction>
                    </AlertDialogFooter>
                </AlertDialogContent>
            </AlertDialog>
        </div>
    )
}
