import { useCallback, useState } from "react"
import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { useNavigate, useParams } from "react-router"
import { useTranslation } from "react-i18next"
import { Clock, Info, RefreshCw, X } from "lucide-react"
import { NIL } from "uuid"
import { toast } from "sonner"
import { courierApi } from "@lunogram-enterprise/oapi-client/courier"
import type { Domain } from "@lunogram-enterprise/oapi-client/courier"
import oapiClient from "@/oapi/client"
import { useResolver } from "../../hooks"
import { StatusBadge, DNSRecordsTable } from "../settings/Domains"
import {
    onboardingDomainSchema,
    type OnboardingDomainFormValues,
} from "@/validation/project/onboarding-domain"

import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
    Card,
    CardContent,
    CardDescription,
    CardFooter,
    CardHeader,
    CardTitle,
} from "@/components/ui/card"
import {
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from "@/components/ui/table"
import type { UUID } from "@/types/common"

const placeholderRecords = [
    {
        type: "MX",
        name: "mail.example.com",
        value: "feedback-smtp.us-east-1.amazonses.com",
        ttl: "Auto",
        priority: "10",
    },
    {
        type: "TXT",
        name: "mail.example.com",
        value: "v=spf1 include:amazonses.com ~all",
        ttl: "Auto",
        priority: "",
    },
    {
        type: "CNAME",
        name: "resend._domainkey.mail.example.com",
        value: "p1.domainkey.example.com",
        ttl: "Auto",
        priority: "",
    },
]

export default function ProjectOnboardingDomain() {
    const navigate = useNavigate()
    const { t } = useTranslation()
    const { projectId = NIL as UUID } = useParams<{ projectId: UUID }>()

    const [isCreating, setIsCreating] = useState(false)

    const form = useForm<OnboardingDomainFormValues>({
        resolver: zodResolver(onboardingDomainSchema),
        mode: "onChange",
        defaultValues: {
            email_address: "",
            display_name: "",
        },
    })
    const [isVerifying, setIsVerifying] = useState(false)
    const [isRemoving, setIsRemoving] = useState(false)
    const [activeDomain, setActiveDomain] = useState<Domain | null>(null)
    const [activeSenderIdentityId, setActiveSenderIdentityId] = useState<string | null>(null)
    const [activeEmail, setActiveEmail] = useState<{ address: string; name?: string } | null>(null)

    // --- Provider gate: find the courier provider ---
    const [courierProvider] = useResolver(
        useCallback(async () => {
            const { data } = await oapiClient.GET("/api/admin/projects/{projectID}/providers", {
                params: { path: { projectID: projectId } },
            })
            const providers = data?.results ?? []
            return providers.find((p) => p.module === "courier") ?? null
        }, [projectId]),
    )

    // --- Load existing domains (returning user) ---
    const [existingDomains] = useResolver(
        useCallback(async () => {
            return await courierApi.domains.list(projectId)
        }, [projectId]),
    )

    // --- Load sender identities so we can show from name/email on refresh ---
    const [senderIdentities] = useResolver(
        useCallback(async () => {
            if (!courierProvider) return null
            const { data } = await oapiClient.GET(
                "/api/admin/projects/{projectID}/sender-identities",
                {
                    params: {
                        path: { projectID: projectId },
                        query: { provider_id: courierProvider.id, channel: "email" },
                    },
                },
            )
            return data?.results ?? []
        }, [projectId, courierProvider]),
    )

    // Show an existing domain if one was already added
    const domain = activeDomain ?? (existingDomains?.length ? existingDomains[0] : null)

    // Derive email info: prefer local state (just-created), fall back to loaded sender identity
    const emailInfo = (() => {
        if (activeEmail) return activeEmail
        if (!domain || !senderIdentities?.length) return null
        const match = senderIdentities.find((si) => {
            const addr = si.traits?.address as string | undefined
            return addr && addr.endsWith("@" + domain.domain_name)
        })
        if (!match) return null
        return {
            address: match.traits.address as string,
            name: (match.traits.name as string) || undefined,
        }
    })()

    // Derive sender identity ID for removal
    const senderIdentityId = (() => {
        if (activeSenderIdentityId) return activeSenderIdentityId
        if (!domain || !senderIdentities?.length) return null
        const match = senderIdentities.find((si) => {
            const addr = si.traits?.address as string | undefined
            return addr && addr.endsWith("@" + domain.domain_name)
        })
        return match?.id ?? null
    })()

    const handleSubmit = form.handleSubmit(async (data) => {
        const address = data.email_address.trim()
        const name = data.display_name?.trim() ?? ""

        setIsCreating(true)

        try {
            // Extract domain from email
            const domainHost = address.split("@")[1]

            // Step 1: Create domain for DNS verification
            const created = await courierApi.domains.create(projectId, domainHost)

            // Step 2: Create sender identity using the courier provider
            const traits: Record<string, unknown> = { address }
            if (name) {
                traits.name = name
            }

            const { data: senderIdentity, error } = await oapiClient.POST(
                "/api/admin/projects/{projectID}/sender-identities",
                {
                    params: { path: { projectID: projectId } },
                    body: {
                        provider_id: courierProvider!.id,
                        channel: "email",
                        traits,
                    },
                },
            )

            if (error) {
                // Domain was created, but sender identity failed — clean up domain
                await courierApi.domains.delete(projectId, created.id).catch(() => {})
                toast.error(t("sender_identity_create_failed", "Failed to create sender identity."))
                return
            }

            setActiveDomain(created)
            setActiveSenderIdentityId(senderIdentity?.id ?? null)
            setActiveEmail({ address, name: name || undefined })
            form.reset()
        } catch {
            toast.error(t("domain_create_failed", "Failed to add domain"))
        } finally {
            setIsCreating(false)
        }
    })

    const handleVerify = async () => {
        if (!domain) return
        setIsVerifying(true)
        try {
            await courierApi.domains.verify(projectId, domain.id)
            // Refresh domain to get updated status
            const refreshed = await courierApi.domains.get(projectId, domain.id)
            setActiveDomain(refreshed)
            toast.success(
                t("domain_verification_triggered", "Verification triggered for {{domain}}", {
                    domain: domain.domain_name,
                }),
            )
        } catch {
            toast.error(t("domain_verify_failed", "Failed to trigger verification"))
        } finally {
            setIsVerifying(false)
        }
    }

    const handleRemove = async () => {
        if (!domain) return
        setIsRemoving(true)
        try {
            // Delete sender identity if we have one
            if (senderIdentityId) {
                await oapiClient
                    .DELETE(
                        "/api/admin/projects/{projectID}/sender-identities/{senderIdentityID}",
                        {
                            params: {
                                path: {
                                    projectID: projectId,
                                    senderIdentityID: senderIdentityId,
                                },
                            },
                        },
                    )
                    .catch(() => {})
            }

            // Delete domain
            await courierApi.domains.delete(projectId, domain.id)

            setActiveDomain(null)
            setActiveSenderIdentityId(null)
            setActiveEmail(null)
        } catch {
            toast.error(t("domain_delete_failed", "Failed to remove domain"))
        } finally {
            setIsRemoving(false)
        }
    }

    async function handleNext() {
        await navigate(`/projects/${projectId}/onboarding/users`)
    }

    const isValidEmail = form.formState.isValid

    // --- Provider gate: no courier provider ---
    // courierProvider is null initially (loading), and null if not found.
    // We distinguish: undefined = still loading, null = not found.
    // useResolver returns null on initial load AND when resolver returns null.
    // To differentiate, we check if providers have been fetched at all.
    const providersLoaded = courierProvider !== null || courierProvider === null
    const hasCourierProvider = courierProvider != null

    // If providers are loaded and no courier provider exists, show info message
    if (
        providersLoaded &&
        !hasCourierProvider &&
        existingDomains !== null &&
        existingDomains.length === 0
    ) {
        return (
            <Card className="w-full max-w-[600px]">
                <CardHeader>
                    <CardTitle className="text-lg">
                        {t("onboarding_domain_title", "Custom Sending Domain")}
                    </CardTitle>
                </CardHeader>
                <CardContent>
                    <div className="flex items-start gap-3 rounded-lg border border-border bg-muted/50 p-4">
                        <Info className="mt-0.5 h-4 w-4 shrink-0 text-muted-foreground" />
                        <p className="text-sm text-muted-foreground">
                            {t(
                                "onboarding_domain_no_provider",
                                "Set up an email integration first to configure a custom sending domain.",
                            )}
                        </p>
                    </div>
                </CardContent>
                <CardFooter>
                    <Button variant="outline" onClick={handleNext}>
                        {t("skip")}
                    </Button>
                </CardFooter>
            </Card>
        )
    }

    return (
        <Card className="w-full max-w-[600px]">
            <CardHeader>
                <CardTitle className="text-lg">
                    {t("onboarding_domain_title", "Custom Sending Domain")}
                </CardTitle>
                <CardDescription>
                    {t(
                        "onboarding_domain_description_email",
                        "Enter the email address you want to send from. We'll set up your domain for email delivery automatically.",
                    )}
                </CardDescription>
            </CardHeader>
            <CardContent>
                {!domain ? (
                    // Email input form + blurred DNS preview
                    <div className="space-y-6">
                        <div className="grid grid-cols-2 gap-3">
                            <div className="space-y-1.5">
                                <Label htmlFor="display-name">
                                    {t("display_name", "Display name")}
                                </Label>
                                <Input
                                    id="display-name"
                                    placeholder="Acme Inc."
                                    {...form.register("display_name")}
                                    onKeyDown={(e) => {
                                        if (e.key === "Enter") {
                                            e.preventDefault()
                                            handleSubmit()
                                        }
                                    }}
                                    disabled={isCreating}
                                />
                            </div>
                            <div className="space-y-1.5">
                                <Label htmlFor="email-address">
                                    {t("email_address", "Email address")}
                                    <span className="text-destructive ml-0.5">*</span>
                                </Label>
                                <Input
                                    id="email-address"
                                    type="email"
                                    placeholder="hello@example.com"
                                    {...form.register("email_address")}
                                    onKeyDown={(e) => {
                                        if (e.key === "Enter") {
                                            e.preventDefault()
                                            handleSubmit()
                                        }
                                    }}
                                    autoFocus
                                    disabled={isCreating}
                                />
                                {form.formState.errors.email_address && (
                                    <p className="text-sm text-destructive">
                                        {form.formState.errors.email_address.message}
                                    </p>
                                )}
                            </div>
                        </div>
                        <div className="relative select-none overflow-hidden" aria-hidden="true">
                            <div className="blur-[2px] opacity-50 pointer-events-none">
                                <div className="rounded-md border">
                                    <Table>
                                        <TableHeader>
                                            <TableRow>
                                                <TableHead className="w-[60px]">Status</TableHead>
                                                <TableHead>Type</TableHead>
                                                <TableHead>Name</TableHead>
                                                <TableHead>Value</TableHead>
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
                                                        <code className="text-xs">
                                                            {record.name}
                                                        </code>
                                                    </TableCell>
                                                    <TableCell>
                                                        <code className="text-xs">
                                                            {record.value}
                                                        </code>
                                                    </TableCell>
                                                </TableRow>
                                            ))}
                                        </TableBody>
                                    </Table>
                                </div>
                            </div>
                        </div>
                    </div>
                ) : (
                    // Domain added — show email + DNS records inline
                    <div className="space-y-4">
                        <div className="flex items-center justify-between">
                            <div className="flex items-center gap-2">
                                <div className="flex flex-col">
                                    {emailInfo?.name && (
                                        <span className="font-medium text-sm">
                                            {emailInfo.name}
                                        </span>
                                    )}
                                    <span
                                        className={`text-sm ${emailInfo?.name ? "text-muted-foreground font-mono text-xs" : "font-medium"}`}
                                    >
                                        {emailInfo?.address ?? domain.domain_name}
                                    </span>
                                </div>
                                <StatusBadge status={domain.status} />
                            </div>
                            <div className="flex items-center gap-1">
                                {domain.status !== "verified" && (
                                    <Button
                                        size="sm"
                                        variant="outline"
                                        disabled={isVerifying}
                                        onClick={handleVerify}
                                    >
                                        <RefreshCw
                                            className={`mr-1.5 h-3.5 w-3.5 ${isVerifying ? "animate-spin" : ""}`}
                                        />
                                        {t("verify", "Verify")}
                                    </Button>
                                )}
                                <Button
                                    size="icon"
                                    variant="ghost"
                                    className="h-8 w-8 text-muted-foreground hover:text-destructive"
                                    onClick={handleRemove}
                                    disabled={isRemoving}
                                    title={t("remove", "Remove")}
                                >
                                    <X className="h-4 w-4" />
                                </Button>
                            </div>
                        </div>

                        {domain.status !== "verified" && (
                            <div className="space-y-2">
                                <p className="text-xs text-muted-foreground">
                                    {t(
                                        "dns_records_hint",
                                        "Add these records to your DNS provider, then click verify.",
                                    )}
                                </p>
                                <DNSRecordsTable records={domain.dns_records} />
                            </div>
                        )}
                    </div>
                )}
            </CardContent>
            <CardFooter className="flex gap-2">
                {!domain ? (
                    <>
                        <Button
                            onClick={() => handleSubmit()}
                            disabled={!isValidEmail}
                            isLoading={isCreating}
                        >
                            {t("set_up_domain", "Set up domain")}
                        </Button>
                        <Button variant="outline" onClick={handleNext}>
                            {t("skip")}
                        </Button>
                    </>
                ) : (
                    <>
                        <Button onClick={handleNext}>{t("next")}</Button>
                        <Button variant="outline" onClick={handleNext}>
                            {t("skip")}
                        </Button>
                    </>
                )}
            </CardFooter>
        </Card>
    )
}
