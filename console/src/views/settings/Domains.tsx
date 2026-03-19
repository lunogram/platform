import { useCallback, useContext, useState } from "react"
import { useTranslation } from "react-i18next"
import {
    Plus,
    Globe,
    Trash2,
    RefreshCw,
    CheckCircle,
    Clock,
    AlertCircle,
    Copy,
    ChevronDown,
    ChevronRight,
} from "lucide-react"
import { toast } from "sonner"
import { ProjectContext } from "../../contexts"
import { useResolver } from "../../hooks"
import { courierApi } from "@lunogram-enterprise/oapi-client/courier"
import type { Domain, DNSRecord } from "@lunogram-enterprise/oapi-client/courier"

import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import {
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from "@/components/ui/table"
import { Badge } from "@/components/ui/badge"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"
import { Skeleton } from "@/components/ui/skeleton"
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from "@/components/ui/dialog"

export function StatusBadge({ status }: { status: string }) {
    switch (status) {
        case "verified":
            return (
                <Badge className="gap-1.5 bg-emerald-500/15 text-emerald-600 border-emerald-500/20 hover:bg-emerald-500/15">
                    <CheckCircle className="h-3 w-3" />
                    Verified
                </Badge>
            )
        case "not_started":
        case "pending":
            return (
                <Tooltip>
                    <TooltipTrigger asChild>
                        <Badge className="gap-1.5 bg-amber-500/15 text-amber-600 border-amber-500/20 hover:bg-amber-500/15">
                            <Clock className="h-3 w-3" />
                            Pending
                        </Badge>
                    </TooltipTrigger>
                    <TooltipContent>Add the DNS records below, then click verify.</TooltipContent>
                </Tooltip>
            )
        case "failed":
            return (
                <Badge variant="destructive" className="gap-1.5">
                    <AlertCircle className="h-3 w-3" />
                    Failed
                </Badge>
            )
        default:
            return (
                <Badge variant="secondary" className="gap-1.5">
                    <Clock className="h-3 w-3" />
                    {status}
                </Badge>
            )
    }
}

function RecordStatusIcon({ status }: { status: string }) {
    if (status === "verified") {
        return <CheckCircle className="h-3.5 w-3.5 text-emerald-500" />
    }
    if (status === "failed") {
        return <AlertCircle className="h-3.5 w-3.5 text-destructive" />
    }
    return <Clock className="h-3.5 w-3.5 text-muted-foreground" />
}

export function DNSRecordsTable({ records }: { records: DNSRecord[] }) {
    const handleCopy = async (value: string) => {
        await navigator.clipboard.writeText(value)
        toast.success("Copied to clipboard")
    }

    if (!records || records.length === 0) {
        return <p className="text-sm text-muted-foreground py-2">No DNS records available.</p>
    }

    return (
        <div className="rounded-md border">
            <Table>
                <TableHeader>
                    <TableRow>
                        <TableHead className="w-[60px]">Status</TableHead>
                        <TableHead>Type</TableHead>
                        <TableHead>Name</TableHead>
                        <TableHead>Value</TableHead>
                        <TableHead className="hidden md:table-cell">TTL</TableHead>
                        <TableHead className="hidden md:table-cell">Priority</TableHead>
                    </TableRow>
                </TableHeader>
                <TableBody>
                    {records.map((record, i) => (
                        <TableRow key={i}>
                            <TableCell>
                                <RecordStatusIcon status={record.status} />
                            </TableCell>
                            <TableCell>
                                <code className="text-xs bg-muted px-1.5 py-0.5 rounded">
                                    {record.type}
                                </code>
                            </TableCell>
                            <TableCell>
                                <div className="flex items-center gap-1.5 max-w-[200px]">
                                    <code className="text-xs truncate">{record.name}</code>
                                    <Button
                                        size="icon"
                                        variant="ghost"
                                        className="h-6 w-6 shrink-0"
                                        onClick={() => handleCopy(record.name)}
                                    >
                                        <Copy className="h-3 w-3" />
                                    </Button>
                                </div>
                            </TableCell>
                            <TableCell>
                                <div className="flex items-center gap-1.5 max-w-[300px]">
                                    <code className="text-xs truncate">{record.value}</code>
                                    <Button
                                        size="icon"
                                        variant="ghost"
                                        className="h-6 w-6 shrink-0"
                                        onClick={() => handleCopy(record.value)}
                                    >
                                        <Copy className="h-3 w-3" />
                                    </Button>
                                </div>
                            </TableCell>
                            <TableCell className="hidden md:table-cell text-muted-foreground text-xs">
                                {record.ttl}
                            </TableCell>
                            <TableCell className="hidden md:table-cell text-muted-foreground text-xs">
                                {record.priority || "\u2014"}
                            </TableCell>
                        </TableRow>
                    ))}
                </TableBody>
            </Table>
        </div>
    )
}

const placeholderRecords = [
    {
        type: "MX",
        name: "mail.example.com",
        value: "feedback-smtp.us-east-1.amazonses.com",
        ttl: "Auto",
        priority: "10",
        status: "pending",
    },
    {
        type: "TXT",
        name: "mail.example.com",
        value: "v=spf1 include:amazonses.com ~all",
        ttl: "Auto",
        priority: "",
        status: "pending",
    },
    {
        type: "CNAME",
        name: "resend._domainkey.mail.example.com",
        value: "p1.domainkey.example.com",
        ttl: "Auto",
        priority: "",
        status: "pending",
    },
]

function EmptyDomainState({ onAdd }: { onAdd: () => void }) {
    const { t } = useTranslation()
    return (
        <div className="flex flex-col items-center gap-6 py-10 px-6">
            <div className="flex flex-col items-center gap-2 text-center max-w-md">
                <Globe className="h-8 w-8 text-muted-foreground/60" />
                <h3 className="text-base font-medium">
                    {t("no_domains_yet", "No custom domains configured yet")}
                </h3>
                <p className="text-sm text-muted-foreground">
                    {t(
                        "domains_empty_description",
                        "Connect a custom domain to send emails from your own address. Once added, you'll receive DNS records to configure with your domain provider.",
                    )}
                </p>
            </div>

            {/* Blurred DNS preview */}
            <div
                className="w-full max-w-2xl relative select-none overflow-hidden"
                aria-hidden="true"
            >
                <div className="blur-[2px] opacity-50 pointer-events-none">
                    <div className="rounded-md border">
                        <Table>
                            <TableHeader>
                                <TableRow>
                                    <TableHead className="w-[60px]">Status</TableHead>
                                    <TableHead>Type</TableHead>
                                    <TableHead>Name</TableHead>
                                    <TableHead>Value</TableHead>
                                    <TableHead className="hidden md:table-cell">TTL</TableHead>
                                    <TableHead className="hidden md:table-cell">Priority</TableHead>
                                </TableRow>
                            </TableHeader>
                            <TableBody>
                                {placeholderRecords.map((record, i) => (
                                    <TableRow key={i}>
                                        <TableCell>
                                            <Clock className="h-3.5 w-3.5 text-muted-foreground" />
                                        </TableCell>
                                        <TableCell>
                                            <code className="text-xs bg-muted px-1.5 py-0.5 rounded">
                                                {record.type}
                                            </code>
                                        </TableCell>
                                        <TableCell>
                                            <code className="text-xs">{record.name}</code>
                                        </TableCell>
                                        <TableCell>
                                            <code className="text-xs">{record.value}</code>
                                        </TableCell>
                                        <TableCell className="hidden md:table-cell text-muted-foreground text-xs">
                                            {record.ttl}
                                        </TableCell>
                                        <TableCell className="hidden md:table-cell text-muted-foreground text-xs">
                                            {record.priority || "\u2014"}
                                        </TableCell>
                                    </TableRow>
                                ))}
                            </TableBody>
                        </Table>
                    </div>
                </div>
            </div>

            <Button size="sm" onClick={onAdd}>
                <Plus className="mr-1.5 h-3.5 w-3.5" />
                {t("add_domain", "Add Domain")}
            </Button>
        </div>
    )
}

/**
 * Reusable domain management component.
 * Renders the domain list, add/verify/delete actions, and DNS records.
 * Used both in the Settings > Domains page and in the IntegrationSetup view.
 */
export function DomainManager({ projectId }: { projectId: string }) {
    const { t } = useTranslation()

    const [isAddingDomain, setIsAddingDomain] = useState(false)
    const [newDomainName, setNewDomainName] = useState("")
    const [isCreating, setIsCreating] = useState(false)
    const [verifyingId, setVerifyingId] = useState<string | null>(null)
    const [deletingId, setDeletingId] = useState<string | null>(null)
    const [expandedId, setExpandedId] = useState<string | null>(null)

    const [domains, , reload] = useResolver(
        useCallback(async () => {
            return await courierApi.domains.list(projectId)
        }, [projectId]),
    )

    const handleCreate = async () => {
        if (!newDomainName.trim()) return
        setIsCreating(true)
        try {
            const created = await courierApi.domains.create(projectId, newDomainName.trim())
            toast.success(t("domain_created", "Domain added successfully"))
            setIsAddingDomain(false)
            setNewDomainName("")
            await reload()
            setExpandedId(created.id)
        } catch {
            toast.error(t("domain_create_failed", "Failed to add domain"))
        } finally {
            setIsCreating(false)
        }
    }

    const handleVerify = async (domain: Domain) => {
        setVerifyingId(domain.id)
        try {
            await courierApi.domains.verify(projectId, domain.id)
            toast.success(
                t("domain_verification_triggered", "Verification triggered for {{domain}}", {
                    domain: domain.domain_name,
                }),
            )
            await reload()
        } catch {
            toast.error(t("domain_verify_failed", "Failed to trigger verification"))
        } finally {
            setVerifyingId(null)
        }
    }

    const handleDelete = async (domain: Domain) => {
        if (!confirm(t("domain_delete_confirm", `Remove domain "${domain.domain_name}"?`))) return
        setDeletingId(domain.id)
        try {
            await courierApi.domains.delete(projectId, domain.id)
            toast.success(
                t("domain_deleted", "Domain {{domain}} removed", {
                    domain: domain.domain_name,
                }),
            )
            if (expandedId === domain.id) setExpandedId(null)
            await reload()
        } catch {
            toast.error(t("domain_delete_failed", "Failed to remove domain"))
        } finally {
            setDeletingId(null)
        }
    }

    const toggleExpand = (domainId: string) => {
        setExpandedId((prev) => (prev === domainId ? null : domainId))
    }

    return (
        <>
            {/* Header */}
            <div className="flex items-center justify-between">
                <div>
                    <h3 className="text-sm font-medium">
                        {t("sending_domains", "Sending Domains")}
                    </h3>
                    <p className="text-xs text-muted-foreground mt-0.5">
                        {t(
                            "domains_description",
                            "Configure custom sending domains for email delivery.",
                        )}
                    </p>
                </div>
                <Button size="sm" variant="outline" onClick={() => setIsAddingDomain(true)}>
                    <Plus className="mr-1.5 h-3.5 w-3.5" />
                    {t("add_domain", "Add Domain")}
                </Button>
            </div>

            {/* Domain List */}
            <div className="rounded-md border">
                <Table>
                    <TableHeader>
                        <TableRow>
                            <TableHead className="w-[30px]" />
                            <TableHead>{t("domain", "Domain")}</TableHead>
                            <TableHead>{t("status", "Status")}</TableHead>
                            <TableHead className="hidden md:table-cell">
                                {t("created", "Created")}
                            </TableHead>
                            <TableHead className="w-[120px]" />
                        </TableRow>
                    </TableHeader>
                    <TableBody>
                        {!domains ? (
                            Array.from({ length: 2 }).map((_, i) => (
                                <TableRow key={i}>
                                    <TableCell />
                                    <TableCell>
                                        <Skeleton className="h-4 w-40" />
                                    </TableCell>
                                    <TableCell>
                                        <Skeleton className="h-5 w-20" />
                                    </TableCell>
                                    <TableCell className="hidden md:table-cell">
                                        <Skeleton className="h-4 w-24" />
                                    </TableCell>
                                    <TableCell>
                                        <Skeleton className="h-8 w-20" />
                                    </TableCell>
                                </TableRow>
                            ))
                        ) : domains.length === 0 ? (
                            <TableRow>
                                <TableCell colSpan={5} className="p-0">
                                    <EmptyDomainState onAdd={() => setIsAddingDomain(true)} />
                                </TableCell>
                            </TableRow>
                        ) : (
                            domains.map((domain) => (
                                <>
                                    <TableRow
                                        key={domain.id}
                                        className="cursor-pointer"
                                        onClick={() => toggleExpand(domain.id)}
                                    >
                                        <TableCell className="w-[30px]">
                                            {expandedId === domain.id ? (
                                                <ChevronDown className="h-4 w-4 text-muted-foreground" />
                                            ) : (
                                                <ChevronRight className="h-4 w-4 text-muted-foreground" />
                                            )}
                                        </TableCell>
                                        <TableCell className="font-medium">
                                            {domain.domain_name}
                                        </TableCell>
                                        <TableCell>
                                            <StatusBadge status={domain.status} />
                                        </TableCell>
                                        <TableCell className="hidden md:table-cell text-muted-foreground text-sm">
                                            {new Date(domain.created_at).toLocaleDateString()}
                                        </TableCell>
                                        <TableCell>
                                            <div
                                                className="flex items-center gap-1"
                                                onClick={(e) => e.stopPropagation()}
                                            >
                                                {domain.status !== "verified" && (
                                                    <Button
                                                        size="icon"
                                                        variant="ghost"
                                                        className="h-8 w-8"
                                                        disabled={verifyingId === domain.id}
                                                        onClick={() => handleVerify(domain)}
                                                        title={t("verify", "Verify")}
                                                    >
                                                        <RefreshCw
                                                            className={`h-4 w-4 ${verifyingId === domain.id ? "animate-spin" : ""}`}
                                                        />
                                                    </Button>
                                                )}
                                                <Button
                                                    size="icon"
                                                    variant="ghost"
                                                    className="h-8 w-8 text-destructive hover:text-destructive"
                                                    disabled={deletingId === domain.id}
                                                    onClick={() => handleDelete(domain)}
                                                    title={t("delete", "Delete")}
                                                >
                                                    <Trash2 className="h-4 w-4" />
                                                </Button>
                                            </div>
                                        </TableCell>
                                    </TableRow>
                                    {expandedId === domain.id && (
                                        <TableRow key={`${domain.id}-dns`}>
                                            <TableCell colSpan={5} className="bg-muted/30 p-4">
                                                <div className="space-y-3">
                                                    <div className="flex items-center justify-between">
                                                        <h4 className="text-sm font-medium">
                                                            {t("dns_records", "DNS Records")}
                                                        </h4>
                                                        <p className="text-xs text-muted-foreground">
                                                            {t(
                                                                "dns_records_hint",
                                                                "Add these records to your DNS provider, then click verify.",
                                                            )}
                                                        </p>
                                                    </div>
                                                    <DNSRecordsTable records={domain.dns_records} />
                                                </div>
                                            </TableCell>
                                        </TableRow>
                                    )}
                                </>
                            ))
                        )}
                    </TableBody>
                </Table>
            </div>

            {/* Add Domain Dialog */}
            <Dialog open={isAddingDomain} onOpenChange={setIsAddingDomain}>
                <DialogContent>
                    <DialogHeader>
                        <DialogTitle>{t("add_domain", "Add Domain")}</DialogTitle>
                        <DialogDescription>
                            {t(
                                "add_domain_description",
                                "Enter your domain name. You will need to add DNS records to verify ownership.",
                            )}
                        </DialogDescription>
                    </DialogHeader>
                    <div className="py-4">
                        <Input
                            placeholder="mail.example.com"
                            value={newDomainName}
                            onChange={(e) => setNewDomainName(e.target.value)}
                            onKeyDown={(e) => {
                                if (e.key === "Enter") handleCreate()
                            }}
                            autoFocus
                        />
                    </div>
                    <DialogFooter>
                        <Button
                            variant="outline"
                            onClick={() => {
                                setIsAddingDomain(false)
                                setNewDomainName("")
                            }}
                        >
                            {t("cancel", "Cancel")}
                        </Button>
                        <Button
                            onClick={handleCreate}
                            disabled={isCreating || !newDomainName.trim()}
                        >
                            {isCreating ? t("adding", "Adding...") : t("add_domain", "Add Domain")}
                        </Button>
                    </DialogFooter>
                </DialogContent>
            </Dialog>
        </>
    )
}

/**
 * Full-page Domains settings view — wraps DomainManager with page-level header.
 */
export default function Domains() {
    const [project] = useContext(ProjectContext)

    return (
        <div className="flex flex-col gap-6">
            <DomainManager projectId={project.id} />
        </div>
    )
}
