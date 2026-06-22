import { useEffect, useState } from "react"
import { Navigate, useNavigate, useParams } from "react-router"
import { toast } from "sonner"
import { ArrowLeft, Check, ChevronDown } from "lucide-react"

import {
    activeConstraints,
    hasVerifiedSubject,
    identityMeta,
    newClient,
    permissionSummary,
    restrictableResources,
    type Client,
    type Identity,
} from "./model"
import { createClient, fetchClient, removeClient, updateClient } from "./store"
import { GeneralFields } from "./sections/GeneralFields"
import { AuthenticationFields } from "./sections/AuthenticationFields"
import { PermissionsFields } from "./sections/PermissionsFields"
import { ClientCreated } from "./ClientCreated"

import { cn, snakeToTitle } from "@/utils"
import { Button } from "@/components/ui/button"

type SectionId = "general" | "authentication" | "permissions"

const SECTIONS: { id: SectionId; title: string; hint: string }[] = [
    { id: "general", title: "General", hint: "Name this client." },
    { id: "authentication", title: "Authentication", hint: "How it proves who it is." },
    { id: "permissions", title: "Permissions", hint: "What it may do, and whose data." },
]

function ClientEditorLayout({ isNew, clientId }: { isNew: boolean; clientId: string }) {
    const navigate = useNavigate()
    const { projectId = "" } = useParams()
    const listPath = `/projects/${projectId}/settings/access`

    const [client, setClient] = useState<Client | null>(() => (isNew ? newClient() : null))
    // Edit mode loads the existing client; a load failure sends us back to the list.
    const [loading, setLoading] = useState(!isNew)
    const [notFound, setNotFound] = useState(false)
    // Guided creation state: which section is open, and how far we've progressed.
    const [open, setOpen] = useState<SectionId | null>("general")
    const [maxStep, setMaxStep] = useState(0)
    const [saving, setSaving] = useState(false)
    // After a new client is created we reveal its one-time secret in place.
    const [created, setCreated] = useState<{ client: Client; secret: string | null } | null>(null)

    useEffect(() => {
        if (isNew) return
        let active = true
        fetchClient(projectId, clientId)
            .then((c) => active && setClient(c))
            .catch(() => active && setNotFound(true))
            .finally(() => active && setLoading(false))
        return () => {
            active = false
        }
    }, [isNew, projectId, clientId])

    if (notFound) return <Navigate to={listPath} replace />

    if (created)
        return (
            <ClientCreated
                client={created.client}
                secret={created.secret}
                onDone={() => navigate(listPath)}
                onCreateAnother={() => {
                    setClient(newClient())
                    setCreated(null)
                    setOpen("general")
                    setMaxStep(0)
                    setSaving(false)
                    window.scrollTo({ top: 0 })
                }}
            />
        )

    if (loading || !client)
        return <div className="py-16 text-center text-sm text-ink-soft">Loading…</div>

    const set = (patch: Partial<Client>) => setClient((c) => (c ? { ...c, ...patch } : c))
    const updateIdentity = (patch: Partial<Identity>) =>
        set({ identity: { ...client.identity, ...patch } })

    const verified = hasVerifiedSubject(client.identity)
    const canSave = !!client.name.trim()

    const save = async () => {
        setSaving(true)
        try {
            await updateClient(projectId, client)
            navigate(listPath)
        } catch {
            toast.error("Couldn't save changes. Please try again.")
            setSaving(false)
        }
    }

    const create = async () => {
        setSaving(true)
        try {
            const result = await createClient(projectId, client)
            setCreated(result)
        } catch {
            toast.error("Couldn't create the client. Please try again.")
            setSaving(false)
        }
    }

    const chipsFor = (id: SectionId): string[] => {
        if (id === "general") return [client.name.trim() || "Untitled client"]
        if (id === "authentication") {
            const chips = [identityMeta[client.identity.type].label]
            if (verified) chips.push(client.subjectScope === "own" ? "Own data" : "All data")
            return chips
        }
        const chips = [
            client.permissions.kind === "role"
                ? snakeToTitle(client.permissions.role)
                : permissionSummary(client.permissions),
        ]
        const constraints = activeConstraints(client)
        for (const resource of restrictableResources) {
            const names = constraints[resource]
            if (names) chips.push(`${snakeToTitle(resource)}: ${names.length}`)
        }
        return chips
    }

    const bodyFor = (id: SectionId) => {
        if (id === "general") return <GeneralFields client={client} set={set} />
        if (id === "authentication")
            return (
                <AuthenticationFields
                    client={client}
                    set={set}
                    updateIdentity={updateIdentity}
                    verified={verified}
                    readOnly={!isNew}
                />
            )
        return <PermissionsFields client={client} set={set} />
    }

    return (
        <div className="flex flex-col gap-8">
            {/* Header */}
            <div className="flex items-start gap-3">
                <Button
                    variant="ghost"
                    size="icon"
                    className="h-8 w-8 shrink-0 text-ink-soft"
                    onClick={() => navigate(listPath)}
                >
                    <ArrowLeft className="h-4 w-4" />
                </Button>
                <div className="min-w-0 flex-1">
                    <h2 className="truncate text-2xl font-semibold tracking-tight">
                        {isNew ? "New client" : client.name || "Untitled client"}
                    </h2>
                    {isNew && (
                        <p className="mt-0.5 text-sm text-ink-soft">
                            Step {maxStep + 1} of {SECTIONS.length}
                        </p>
                    )}
                </div>
                <div className="flex shrink-0 items-center gap-2">
                    <Button variant="outline" onClick={() => navigate(listPath)}>
                        Cancel
                    </Button>
                    {!isNew && (
                        <Button onClick={save} disabled={!canSave || saving}>
                            Save changes
                        </Button>
                    )}
                </div>
            </div>

            {/* One cohesive panel; each section is a toned header band over a white body. */}
            <div className="max-w-6xl divide-y overflow-hidden rounded-xl border bg-card">
                {SECTIONS.map((section, i) => {
                    const isLast = i === SECTIONS.length - 1
                    // In edit mode every section is open; creation is guided.
                    const isOpen = isNew ? open === section.id : true
                    const done = isNew && i < maxStep
                    const locked = isNew && i > maxStep

                    return (
                        <section key={section.id}>
                            <SectionHeader
                                title={section.title}
                                hint={section.hint}
                                open={isOpen}
                                done={done}
                                locked={locked}
                                showState={isNew}
                                chips={!isOpen && done ? chipsFor(section.id) : undefined}
                                onToggle={
                                    isNew && !locked
                                        ? () => setOpen(isOpen ? null : section.id)
                                        : undefined
                                }
                            />

                            {isOpen && (
                                <div className="animate-in fade-in-0 slide-in-from-top-1 border-t bg-card px-5 py-6 duration-200 sm:px-6">
                                    {bodyFor(section.id)}
                                    {isNew && (
                                        <div className="mt-7">
                                            {isLast ? (
                                                <Button
                                                    onClick={create}
                                                    disabled={!canSave || saving}
                                                >
                                                    Create client
                                                </Button>
                                            ) : (
                                                <Button
                                                    disabled={section.id === "general" && !canSave}
                                                    onClick={() => {
                                                        setMaxStep((m) => Math.max(m, i + 1))
                                                        setOpen(SECTIONS[i + 1].id)
                                                    }}
                                                >
                                                    Continue
                                                </Button>
                                            )}
                                        </div>
                                    )}
                                </div>
                            )}
                        </section>
                    )
                })}
            </div>

            {!isNew && (
                <div className="flex max-w-6xl items-center justify-between gap-4 rounded-xl border border-red-soft bg-red-soft/15 px-5 py-4">
                    <div className="grid gap-0.5">
                        <span className="text-sm font-medium">Delete this client</span>
                        <span className="text-xs text-ink-soft">
                            Revokes its access immediately. This cannot be undone.
                        </span>
                    </div>
                    <Button
                        variant="outline"
                        disabled={saving}
                        className="shrink-0 border-red-soft text-red hover:bg-red-soft/40 hover:text-red"
                        onClick={async () => {
                            setSaving(true)
                            try {
                                await removeClient(projectId, client.id)
                                navigate(listPath)
                            } catch {
                                toast.error("Couldn't delete the client. Please try again.")
                                setSaving(false)
                            }
                        }}
                    >
                        Delete client
                    </Button>
                </div>
            )}
        </div>
    )
}

function SectionHeader({
    title,
    hint,
    open,
    done,
    locked,
    showState,
    chips,
    onToggle,
}: {
    title: string
    hint: string
    open: boolean
    done: boolean
    locked: boolean
    showState: boolean
    chips?: string[]
    onToggle?: () => void
}) {
    const inner = (
        <>
            <div className="flex min-w-0 items-center gap-3">
                {showState && (done ? <CheckBadge /> : <Dot active={open && !locked} />)}
                <div className="grid min-w-0 gap-0.5">
                    <span className={cn("font-semibold tracking-tight", locked && "text-ink-soft")}>
                        {title}
                    </span>
                    {open && <span className="text-sm font-normal text-ink-soft">{hint}</span>}
                </div>
            </div>
            <div className="flex shrink-0 items-center gap-3">
                {chips && <Chips items={chips} />}
                {onToggle && (
                    <ChevronDown
                        className={cn(
                            "h-4 w-4 text-ink-soft transition-transform",
                            open && "rotate-180",
                        )}
                    />
                )}
            </div>
        </>
    )

    const className =
        "flex w-full items-center justify-between gap-4 bg-surface-soft px-5 py-4 text-left sm:px-6"

    if (onToggle) {
        return (
            <button
                type="button"
                onClick={onToggle}
                className={cn(className, "transition-colors hover:bg-surface-muted")}
            >
                {inner}
            </button>
        )
    }
    return <div className={className}>{inner}</div>
}

function CheckBadge() {
    return (
        <span className="flex h-5 w-5 shrink-0 items-center justify-center rounded-full bg-primary text-surface-fixed">
            <Check className="h-3 w-3" strokeWidth={3} />
        </span>
    )
}

function Dot({ active }: { active?: boolean }) {
    return (
        <span
            className={cn(
                "h-5 w-5 shrink-0 rounded-full border-2",
                active ? "border-primary" : "border-border-strong",
            )}
        />
    )
}

function Chips({ items }: { items: string[] }) {
    return (
        <div className="hidden items-center gap-1.5 sm:flex">
            {items.map((item) => (
                <span
                    key={item}
                    className="rounded bg-surface-muted px-2 py-0.5 text-xs font-medium text-ink-soft"
                >
                    {item}
                </span>
            ))}
        </div>
    )
}

// Route wrappers — the key forces a fresh draft when switching between clients
// (or to the new-client form), since the element is otherwise reused.
export function NewClientRoute() {
    return <ClientEditorLayout key="new" isNew clientId="" />
}

export function EditClientRoute() {
    const { clientId = "" } = useParams()
    return <ClientEditorLayout key={clientId} isNew={false} clientId={clientId} />
}
