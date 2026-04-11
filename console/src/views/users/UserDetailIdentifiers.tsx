import React, { useContext, useState } from "react"
import { useTranslation } from "react-i18next"
import { ChevronDown, ChevronRight, Trash2, Fingerprint, Plus, Save } from "lucide-react"
import { toast } from "sonner"
import { ProjectContext, UserContext } from "../../contexts"
import { PreferencesContext } from "@/contexts/PreferencesContext"
import { formatDate, cn } from "../../utils"
import oapiClient from "../../oapi/client"
import api from "../../api"

import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { AttributeEditor } from "@/components/ui/attribute-editor"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
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

export default function UserDetailIdentifiers() {
    const { t } = useTranslation()
    const [project] = useContext(ProjectContext)
    const [user, setUser] = useContext(UserContext)
    const [preferences] = useContext(PreferencesContext)

    const [expandedIdentifierId, setExpandedIdentifierId] = useState<string | null>(null)
    const [editedMetadata, setEditedMetadata] = useState<Record<string, Record<string, unknown>>>(
        {},
    )
    const [savingId, setSavingId] = useState<string | null>(null)
    const [deletingId, setDeletingId] = useState<string | null>(null)
    const [identifierToDelete, setIdentifierToDelete] = useState<string | null>(null)

    // Add identifier dialog state
    const [isAddOpen, setIsAddOpen] = useState(false)
    const [isAdding, setIsAdding] = useState(false)
    const [newSource, setNewSource] = useState("default")
    const [newExternalId, setNewExternalId] = useState("")
    const [newMetadata, setNewMetadata] = useState<Record<string, unknown>>({})

    const identifiers = user.identifier ?? []

    const toggleExpand = (identifierId: string) => {
        setExpandedIdentifierId(expandedIdentifierId === identifierId ? null : identifierId)
    }

    const getMetadata = (identifierId: string): Record<string, unknown> => {
        if (identifierId in editedMetadata) {
            return editedMetadata[identifierId]
        }
        const identifier = identifiers.find((i) => i.id === identifierId)
        return (identifier?.metadata as Record<string, unknown>) ?? {}
    }

    const hasChanges = (identifierId: string): boolean => {
        if (!(identifierId in editedMetadata)) return false
        const identifier = identifiers.find((i) => i.id === identifierId)
        const original = (identifier?.metadata as Record<string, unknown>) ?? {}
        return JSON.stringify(editedMetadata[identifierId]) !== JSON.stringify(original)
    }

    const handleMetadataChange = (identifierId: string, data: Record<string, unknown>) => {
        setEditedMetadata((prev) => ({ ...prev, [identifierId]: data }))
    }

    const handleDiscard = (identifierId: string) => {
        setEditedMetadata((prev) => {
            const next = { ...prev }
            delete next[identifierId]
            return next
        })
    }

    const handleSave = async (identifierId: string) => {
        const identifier = identifiers.find((i) => i.id === identifierId)
        if (!identifier) return

        const updatedMetadata = editedMetadata[identifierId]
        setSavingId(identifierId)
        try {
            const response = await oapiClient.POST(
                "/api/admin/projects/{projectID}/subjects/users",
                {
                    params: {
                        path: {
                            projectID: project.id,
                        },
                    },
                    body: {
                        identifier: [
                            {
                                source: identifier.source,
                                external_id: identifier.external_id,
                                metadata: updatedMetadata,
                            },
                        ],
                    },
                },
            )
            if (response.error) {
                toast.error(t("identifier_save_error", "Failed to save identifier metadata"))
                return
            }
            const updatedUser = await api.users.get(project.id, user.id)
            if (updatedUser) {
                setUser(updatedUser)
            }
            handleDiscard(identifierId)
            toast.success(t("identifier_saved", "Identifier metadata saved"))
        } catch {
            toast.error(t("identifier_save_error", "Failed to save identifier metadata"))
        } finally {
            setSavingId(null)
        }
    }

    const handleDelete = async () => {
        const identifierId = identifierToDelete
        if (!identifierId || identifiers.length <= 1) return

        setDeletingId(identifierId)
        try {
            const response = await oapiClient.DELETE(
                "/api/admin/projects/{projectID}/subjects/users/{userID}/identifiers/{identifierID}",
                {
                    params: {
                        path: {
                            projectID: project.id,
                            userID: user.id,
                            identifierID: identifierId,
                        },
                    },
                },
            )
            if (response.error) {
                toast.error(t("identifier_delete_error", "Failed to delete identifier"))
                return
            }
            if (expandedIdentifierId === identifierId) {
                setExpandedIdentifierId(null)
            }
            handleDiscard(identifierId)
            const updatedUser = await api.users.get(project.id, user.id)
            if (updatedUser) {
                setUser(updatedUser)
            }
            toast.success(t("identifier_deleted", "Identifier deleted"))
        } catch {
            toast.error(t("identifier_delete_error", "Failed to delete identifier"))
        } finally {
            setDeletingId(null)
            setIdentifierToDelete(null)
        }
    }

    const handleAdd = async () => {
        if (!newSource.trim() || !newExternalId.trim()) return

        setIsAdding(true)
        try {
            const response = await oapiClient.POST(
                "/api/admin/projects/{projectID}/subjects/users",
                {
                    params: {
                        path: {
                            projectID: project.id,
                        },
                    },
                    body: {
                        identifier: [
                            // Include existing identifiers so we target the right user
                            ...identifiers.map((id) => ({
                                source: id.source,
                                external_id: id.external_id,
                            })),
                            {
                                source: newSource.trim(),
                                external_id: newExternalId.trim(),
                                metadata:
                                    Object.keys(newMetadata).length > 0 ? newMetadata : undefined,
                            },
                        ],
                    },
                },
            )
            if (response.error) {
                toast.error(t("identifier_add_error", "Failed to add identifier"))
                return
            }
            const updatedUser = await api.users.get(project.id, user.id)
            if (updatedUser) {
                setUser(updatedUser)
            }
            setIsAddOpen(false)
            setNewSource("default")
            setNewExternalId("")
            setNewMetadata({})
            toast.success(t("identifier_added", "Identifier added"))
        } catch {
            toast.error(t("identifier_add_error", "Failed to add identifier"))
        } finally {
            setIsAdding(false)
        }
    }

    return (
        <div className="space-y-3">
            <div className="flex items-start justify-between gap-4">
                <div>
                    <h2 className="text-base font-medium">
                        <Fingerprint className="inline h-4 w-4 mr-1.5 -mt-0.5" />
                        {t("identifiers", "Identifiers")}
                    </h2>
                    <p className="text-sm text-muted-foreground mt-0.5">
                        {t(
                            "identifiers_description",
                            "External identifiers used to identify this user across systems",
                        )}
                    </p>
                </div>
                <Button
                    variant="outline"
                    size="sm"
                    onClick={() => setIsAddOpen(true)}
                    className="shrink-0"
                >
                    <Plus className="h-4 w-4 mr-2" />
                    {t("add", "Add")}
                </Button>
            </div>

            {identifiers.length > 0 && (
                <div className="border rounded-lg">
                    <Table>
                        <TableHeader>
                            <TableRow>
                                <TableHead className="w-8" />
                                <TableHead>{t("source", "Source")}</TableHead>
                                <TableHead>{t("external_id", "External ID")}</TableHead>
                                <TableHead>{t("metadata", "Metadata")}</TableHead>
                                <TableHead className="w-10" />
                            </TableRow>
                        </TableHeader>
                        <TableBody>
                            {identifiers.map((identifier) => {
                                const isExpanded = expandedIdentifierId === identifier.id
                                const metadataKeys = Object.keys(
                                    (identifier.metadata as Record<string, unknown>) ?? {},
                                )
                                const metadataCount = metadataKeys.length

                                return (
                                    <React.Fragment key={identifier.id}>
                                        <TableRow
                                            className={cn(
                                                "cursor-pointer group",
                                                isExpanded && "bg-muted/50",
                                            )}
                                            onClick={() => toggleExpand(identifier.id)}
                                        >
                                            <TableCell className="p-0 pl-3">
                                                {isExpanded ? (
                                                    <ChevronDown className="h-4 w-4 text-muted-foreground" />
                                                ) : (
                                                    <ChevronRight className="h-4 w-4 text-muted-foreground" />
                                                )}
                                            </TableCell>
                                            <TableCell>
                                                <code className="text-xs bg-muted px-1.5 py-0.5 rounded font-mono">
                                                    {identifier.source}
                                                </code>
                                            </TableCell>
                                            <TableCell>
                                                <code className="text-sm font-mono">
                                                    {identifier.external_id}
                                                </code>
                                            </TableCell>
                                            <TableCell>
                                                {metadataCount > 0 ? (
                                                    <span className="text-xs text-muted-foreground bg-muted px-2 py-0.5 rounded">
                                                        {metadataCount}
                                                    </span>
                                                ) : (
                                                    <span className="text-muted-foreground">—</span>
                                                )}
                                            </TableCell>
                                            <TableCell className="p-0 pr-2">
                                                {identifiers.length <= 1 ? (
                                                    <Tooltip>
                                                        <TooltipTrigger asChild>
                                                            <span className="inline-flex opacity-0 group-hover:opacity-100 transition-opacity">
                                                                <Button
                                                                    variant="ghost"
                                                                    size="icon"
                                                                    className="h-7 w-7 shrink-0 text-muted-foreground/40 cursor-not-allowed"
                                                                    disabled
                                                                    onClick={(e) =>
                                                                        e.stopPropagation()
                                                                    }
                                                                >
                                                                    <Trash2 className="h-3.5 w-3.5" />
                                                                </Button>
                                                            </span>
                                                        </TooltipTrigger>
                                                        <TooltipContent>
                                                            {t(
                                                                "last_identifier_tooltip",
                                                                "At least one identifier is required. Add another before removing this one.",
                                                            )}
                                                        </TooltipContent>
                                                    </Tooltip>
                                                ) : (
                                                    <Button
                                                        variant="ghost"
                                                        size="icon"
                                                        className="h-7 w-7 shrink-0 opacity-0 group-hover:opacity-100 text-muted-foreground hover:text-destructive transition-opacity"
                                                        disabled={deletingId === identifier.id}
                                                        onClick={(e) => {
                                                            e.stopPropagation()
                                                            setIdentifierToDelete(identifier.id)
                                                        }}
                                                    >
                                                        <Trash2 className="h-3.5 w-3.5" />
                                                    </Button>
                                                )}
                                            </TableCell>
                                        </TableRow>

                                        {/* Expanded Row */}
                                        {isExpanded && (
                                            <TableRow className="bg-muted/30 hover:bg-muted/30">
                                                <TableCell colSpan={5} className="p-0">
                                                    <div className="px-4 sm:px-6 py-4 space-y-4">
                                                        {/* Identifier Info */}
                                                        <div className="flex flex-col sm:flex-row gap-4 sm:gap-8">
                                                            <div className="space-y-1">
                                                                <p className="text-xs font-medium text-muted-foreground uppercase tracking-wider">
                                                                    {t("id", "ID")}
                                                                </p>
                                                                <code className="text-sm">
                                                                    {identifier.id}
                                                                </code>
                                                            </div>
                                                            <div className="space-y-1">
                                                                <p className="text-xs font-medium text-muted-foreground uppercase tracking-wider">
                                                                    {t("created_at", "Created")}
                                                                </p>
                                                                <p className="text-sm">
                                                                    {formatDate(
                                                                        preferences,
                                                                        identifier.created_at,
                                                                        "PPpp",
                                                                    )}
                                                                </p>
                                                            </div>
                                                            <div className="space-y-1">
                                                                <p className="text-xs font-medium text-muted-foreground uppercase tracking-wider">
                                                                    {t("updated_at", "Updated")}
                                                                </p>
                                                                <p className="text-sm">
                                                                    {formatDate(
                                                                        preferences,
                                                                        identifier.updated_at,
                                                                        "PPpp",
                                                                    )}
                                                                </p>
                                                            </div>
                                                        </div>

                                                        {/* Metadata Header with Save/Discard */}
                                                        <div className="flex items-center justify-between">
                                                            <p className="text-xs font-medium text-muted-foreground uppercase tracking-wider">
                                                                {t("metadata", "Metadata")}
                                                            </p>
                                                            {hasChanges(identifier.id) && (
                                                                <div className="flex items-center gap-3">
                                                                    <span className="text-sm text-amber-600 dark:text-amber-500">
                                                                        {t(
                                                                            "unsaved_changes",
                                                                            "Unsaved changes",
                                                                        )}
                                                                    </span>
                                                                    <Button
                                                                        size="sm"
                                                                        onClick={(e) => {
                                                                            e.stopPropagation()
                                                                            handleSave(
                                                                                identifier.id,
                                                                            )
                                                                        }}
                                                                        disabled={
                                                                            savingId ===
                                                                            identifier.id
                                                                        }
                                                                    >
                                                                        <Save className="h-4 w-4 mr-2" />
                                                                        {savingId === identifier.id
                                                                            ? t(
                                                                                  "saving",
                                                                                  "Saving...",
                                                                              )
                                                                            : t("save", "Save")}
                                                                    </Button>
                                                                </div>
                                                            )}
                                                        </div>

                                                        {/* Metadata Editor */}
                                                        <AttributeEditor
                                                            value={getMetadata(identifier.id)}
                                                            onChange={(data) =>
                                                                handleMetadataChange(
                                                                    identifier.id,
                                                                    data,
                                                                )
                                                            }
                                                            emptyTitle={t(
                                                                "no_metadata",
                                                                "No metadata",
                                                            )}
                                                            emptyDescription={t(
                                                                "no_identifier_metadata_description",
                                                                "Add custom metadata to this identifier.",
                                                            )}
                                                        />
                                                    </div>
                                                </TableCell>
                                            </TableRow>
                                        )}
                                    </React.Fragment>
                                )
                            })}
                        </TableBody>
                    </Table>
                </div>
            )}

            {/* Delete Confirmation Dialog */}
            <Dialog
                open={identifierToDelete !== null}
                onOpenChange={(open) => !open && setIdentifierToDelete(null)}
            >
                <DialogContent>
                    <DialogHeader>
                        <DialogTitle>{t("delete_identifier", "Delete identifier")}</DialogTitle>
                        <DialogDescription>
                            {t(
                                "delete_identifier_warning",
                                "Are you sure you want to remove this identifier? This action cannot be undone.",
                            )}
                        </DialogDescription>
                    </DialogHeader>
                    {identifierToDelete &&
                        (() => {
                            const ident = identifiers.find((i) => i.id === identifierToDelete)
                            if (!ident) return null
                            return (
                                <div className="py-4">
                                    <div className="flex items-center gap-3 p-3 rounded-lg bg-muted">
                                        <div className="flex h-10 w-10 items-center justify-center rounded-lg shrink-0 bg-destructive/10">
                                            <Fingerprint className="h-5 w-5 text-destructive" />
                                        </div>
                                        <div className="min-w-0">
                                            <p className="font-medium font-mono text-sm truncate">
                                                {ident.external_id}
                                            </p>
                                            <p className="text-sm text-muted-foreground">
                                                {t("source", "Source")}:{" "}
                                                <code className="text-xs bg-background px-1 py-0.5 rounded">
                                                    {ident.source}
                                                </code>
                                            </p>
                                        </div>
                                    </div>
                                </div>
                            )
                        })()}
                    <DialogFooter>
                        <Button
                            variant="outline"
                            onClick={() => setIdentifierToDelete(null)}
                            disabled={deletingId !== null}
                        >
                            {t("cancel", "Cancel")}
                        </Button>
                        <Button
                            variant="destructive"
                            onClick={handleDelete}
                            disabled={deletingId !== null}
                        >
                            {deletingId
                                ? t("deleting", "Deleting...")
                                : t("delete_identifier", "Delete identifier")}
                        </Button>
                    </DialogFooter>
                </DialogContent>
            </Dialog>

            {/* Add Identifier Dialog */}
            <Dialog open={isAddOpen} onOpenChange={setIsAddOpen}>
                <DialogContent className="sm:max-w-2xl max-h-[90vh] overflow-y-auto">
                    <DialogHeader>
                        <DialogTitle>{t("add_identifier", "Add identifier")}</DialogTitle>
                        <DialogDescription>
                            {t(
                                "add_identifier_description",
                                "Add a new external identifier to this user.",
                            )}
                        </DialogDescription>
                    </DialogHeader>
                    <div className="grid gap-4 py-4">
                        <div className="grid sm:grid-cols-2 gap-4">
                            <div className="grid gap-2 content-start">
                                <Label htmlFor="new-source">{t("source", "Source")} *</Label>
                                <Input
                                    id="new-source"
                                    placeholder={t(
                                        "source_placeholder",
                                        "e.g., default, stripe, hubspot",
                                    )}
                                    value={newSource}
                                    onChange={(e) => setNewSource(e.target.value)}
                                />
                            </div>
                            <div className="grid gap-2 content-start">
                                <Label htmlFor="new-external-id">
                                    {t("external_id", "External ID")} *
                                </Label>
                                <Input
                                    id="new-external-id"
                                    placeholder={t(
                                        "external_id_placeholder",
                                        "e.g., usr_123, cus_abc",
                                    )}
                                    value={newExternalId}
                                    onChange={(e) => setNewExternalId(e.target.value)}
                                />
                            </div>
                        </div>
                        <div className="grid gap-2">
                            <Label>{t("metadata", "Metadata")}</Label>
                            <AttributeEditor
                                value={newMetadata}
                                onChange={setNewMetadata}
                                emptyTitle={t("no_metadata", "No metadata")}
                                emptyDescription={t(
                                    "add_identifier_metadata_description",
                                    "Optionally add metadata to this identifier.",
                                )}
                            />
                        </div>
                    </div>
                    <DialogFooter>
                        <Button
                            variant="outline"
                            onClick={() => setIsAddOpen(false)}
                            disabled={isAdding}
                        >
                            {t("cancel", "Cancel")}
                        </Button>
                        <Button
                            onClick={handleAdd}
                            disabled={!newSource.trim() || !newExternalId.trim() || isAdding}
                        >
                            {isAdding
                                ? t("adding", "Adding...")
                                : t("add_identifier", "Add identifier")}
                        </Button>
                    </DialogFooter>
                </DialogContent>
            </Dialog>
        </div>
    )
}
