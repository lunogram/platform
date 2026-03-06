/* eslint-disable react-refresh/only-export-components */
import type { JourneyStepType, Rule, RulePath } from "../../../types"
import { EntranceStepIcon } from "../../../components/icons"
import RuleBuilder from "../../users/rules/RuleBuilder"
import { useCallback, useContext, useMemo, useState } from "react"
import { PreferencesContext } from "@/contexts/PreferencesContext"
import { Switch } from "@/components/ui/switch"
import { Label } from "@/components/ui/label"
import { Combobox } from "@/components/ui/combobox"
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs"
import api from "../../../api"
import { useResolver } from "../../../hooks"
import { Button } from "@/components/ui/button"
import { env } from "../../../config/env"
import { useTranslation, Trans } from "react-i18next"
import { createSimpleEventRule, isEventWrapper } from "../../users/rules/RuleHelpers"
import { ruleDescription } from "../../users/rules/RuleDescriptions"
import { highlightSearch } from "@/lib/ui-utils"
import { Badge } from "@/components/ui/badge"
import { Webhook, Zap, Copy, Check } from "lucide-react"
import { Link } from "react-router"
import type { UUID } from "@/types/common"

interface EntranceConfig {
    trigger: "none" | "event"

    // event based
    event_name?: string
    rule?: Rule
    multiple?: boolean
    concurrent?: boolean

    references?: Array<{ id: UUID; name: string }>
}

const triggers = ["none", "event"] as const

const triggerConfig = {
    none: { icon: Webhook, label: "API" },
    event: { icon: Zap, label: "Event" },
} as const

const codeExample = (journeyId: UUID, entranceId: UUID) => `curl --request POST \\
--url '${env.api.baseURL}/client/journeys/${journeyId}/trigger' \\
--header 'Authorization: Bearer API_KEY' \\
--header 'Content-Type: application/json' \\
--data '{
    "entrance_id": ${entranceId},
    "user": {
        "external_id": "example-user-id",
        "extra_user_property": true
    },
    "event": {
        "purchase_amount": 29.99
    }
}'`

function ApiTriggerSection({ journeyId, stepId }: { journeyId: UUID; stepId: UUID }) {
    const { t } = useTranslation()
    const [copied, setCopied] = useState(false)
    const code = codeExample(journeyId, stepId)

    const handleCopy = async () => {
        await navigator.clipboard.writeText(code)
        setCopied(true)
        setTimeout(() => setCopied(false), 2000)
    }

    return (
        <div className="space-y-3">
            <div className="space-y-1">
                <Label className="text-sm font-medium">{t("entrance_trigger")}</Label>
                <p className="text-xs text-muted-foreground">
                    <Trans i18nKey="entrance_trigger_desc">
                        This entrance can be triggered directly via API. An example request is
                        available below. Data from the{" "}
                        <code className="rounded bg-muted px-1 py-0.5 font-mono text-[11px]">
                            event
                        </code>{" "}
                        field will be available for use in the journey and campaign templates under{" "}
                        <code className="rounded bg-muted px-1 py-0.5 font-mono text-[11px]">
                            journey.DATA_KEY_OF_THIS_STEP.*
                        </code>{" "}
                        (for example,{" "}
                        <code className="rounded bg-muted px-1 py-0.5 font-mono text-[11px]">
                            journey.my_entrance.purchaseAmount
                        </code>
                        ).
                    </Trans>
                </p>
            </div>
            <div className="rounded-lg border overflow-hidden">
                <div className="flex items-center justify-between bg-muted/50 px-3 py-2 border-b">
                    <span className="text-xs font-medium text-muted-foreground">cURL</span>
                    <Button
                        variant="ghost"
                        size="sm"
                        className="h-6 gap-1.5 px-2 text-xs text-muted-foreground"
                        onClick={handleCopy}
                    >
                        {copied ? (
                            <>
                                <Check className="h-3 w-3 text-green-500" />
                                Copied
                            </>
                        ) : (
                            <>
                                <Copy className="h-3 w-3" />
                                Copy
                            </>
                        )}
                    </Button>
                </div>
                <pre className="overflow-x-auto p-3 text-xs leading-relaxed font-mono">{code}</pre>
            </div>
        </div>
    )
}

export const entranceStep: JourneyStepType<EntranceConfig> = {
    name: "entrance",
    icon: <EntranceStepIcon />,
    category: "entrance",
    description: "entrance_desc",
    newData: async () => ({
        trigger: "none",
    }),
    Describe({ value: { trigger, event_name, rule, references = [] } }) {
        const { t } = useTranslation()
        const [preferences] = useContext(PreferencesContext)

        if (trigger === "event") {
            const hasConditions = rule && isEventWrapper(rule) && !!rule?.children?.length
            return (
                <div className="space-y-1.5 max-w-[300px]">
                    <div className="flex items-center gap-1.5 text-sm">
                        <Zap className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                        <span>
                            {t("entrance_add_everyone_when") + " "}
                            <strong>{event_name || <>&#8211;</>}</strong>
                            {t("entrance_occurs")}
                        </span>
                    </div>
                    {hasConditions && (
                        <div className="border-l-2 border-muted pl-2 text-xs text-muted-foreground">
                            {rule.children!.map((child, i) => (
                                <span key={i}>
                                    {i > 0 && <>{rule.operator === "and" ? " and " : " or "}</>}
                                    {ruleDescription(preferences, child)}
                                </span>
                            ))}
                        </div>
                    )}
                </div>
            )
        }

        if (references.length) {
            return (
                <div className="space-y-1.5">
                    <div className="flex items-center gap-1.5 text-sm text-muted-foreground">
                        {t("entrance_links")}
                    </div>
                    <div className="flex flex-wrap gap-1">
                        {references.map((journey) => (
                            <Badge variant="secondary" key={journey.id}>
                                <Link to={`../journeys/${journey.id}`}>{journey.name}</Link>
                            </Badge>
                        ))}
                    </div>
                </div>
            )
        }

        return (
            <div className="flex items-center gap-1.5 text-sm text-muted-foreground">
                <Webhook className="h-3.5 w-3.5 shrink-0" />
                {t("entrance_empty")}
            </div>
        )
    },
    Edit({ onChange, project: { id: projectId }, journey: { id: journeyId }, stepId, value }) {
        const { t } = useTranslation()

        const [suggestions] = useResolver(
            useCallback(async () => await api.projects.pathSuggestions(projectId), [projectId]),
        )

        const eventOptions: RulePath[] = useMemo(
            () =>
                Array.isArray(suggestions?.eventPaths)
                    ? suggestions.eventPaths.map((event, index) => ({
                          id: `event-${index}`,
                          name: event.name,
                          path: event.name,
                          type: "event" as const,
                          data_type: "string" as const,
                          visibility: "public" as const,
                      }))
                    : [],
            [suggestions],
        )

        return (
            <>
                <div className="space-y-1.5">
                    <Label className="text-sm font-medium">{t("trigger")}</Label>
                    <Tabs
                        value={value.trigger}
                        onValueChange={(trigger) =>
                            onChange({
                                ...value,
                                trigger: trigger as EntranceConfig["trigger"],
                            })
                        }
                    >
                        <TabsList className="w-full">
                            {triggers.map((key) => {
                                const { icon: Icon, label } = triggerConfig[key]
                                return (
                                    <TabsTrigger key={key} value={key} className="flex-1 gap-1.5">
                                        <Icon className="h-3.5 w-3.5" />
                                        {label}
                                    </TabsTrigger>
                                )
                            })}
                        </TabsList>
                    </Tabs>
                </div>
                {value.trigger === "event" && (
                    <>
                        <div className="space-y-1.5">
                            <Label className="inline-flex items-center gap-0.5 text-sm font-medium">
                                {t("event_name")}
                                <span className="text-destructive">*</span>
                            </Label>
                            <Combobox
                                value={value.event_name ?? ""}
                                onValueChange={(event_name) => {
                                    const currentRule = value.rule
                                    const updatedRule = currentRule
                                        ? { ...currentRule, value: event_name }
                                        : createSimpleEventRule(event_name)
                                    onChange({
                                        ...value,
                                        event_name,
                                        rule: updatedRule,
                                    })
                                }}
                                options={eventOptions}
                                placeholder={t("event_name")}
                                required
                                renderOption={(option, search) => (
                                    <span
                                        dangerouslySetInnerHTML={{
                                            __html: highlightSearch(option.name, search),
                                        }}
                                    />
                                )}
                            />
                        </div>
                        {value.event_name && (
                            <RuleBuilder
                                rule={value.rule ?? createSimpleEventRule(value.event_name)}
                                setRule={(rule) =>
                                    onChange({
                                        ...value,
                                        rule: {
                                            ...rule,
                                            value: value.event_name,
                                        },
                                    })
                                }
                                eventName={value.event_name}
                                headerPrefix={t("entrance_matching")}
                            />
                        )}
                        <div className="flex items-center justify-between gap-4 rounded-lg border p-4">
                            <div className="space-y-0.5">
                                <Label htmlFor="multiple" className="text-sm font-medium">
                                    {t("entrance_multiple_entries")}
                                </Label>
                                <p className="text-xs text-muted-foreground">
                                    {t("entrance_multiple_entries_desc")}
                                </p>
                            </div>
                            <Switch
                                id="multiple"
                                checked={Boolean(value.multiple)}
                                onCheckedChange={(multiple) => onChange({ ...value, multiple })}
                            />
                        </div>
                        {value.multiple && (
                            <div className="flex items-center justify-between gap-4 rounded-lg border p-4">
                                <div className="space-y-0.5">
                                    <Label htmlFor="concurrent" className="text-sm font-medium">
                                        {t("entrance_simultaneous_entries")}
                                    </Label>
                                    <p className="text-xs text-muted-foreground">
                                        {t("entrance_simultaneous_entries_desc")}
                                    </p>
                                </div>
                                <Switch
                                    id="concurrent"
                                    checked={Boolean(value.concurrent)}
                                    onCheckedChange={(concurrent) =>
                                        onChange({ ...value, concurrent })
                                    }
                                />
                            </div>
                        )}
                    </>
                )}
                {!!stepId && value.trigger === "none" && (
                    <ApiTriggerSection journeyId={journeyId} stepId={stepId} />
                )}
            </>
        )
    },
    validate: ({ trigger, event_name }) => {
        if (trigger === "event") {
            return !!event_name
        }
        return true
    },
    hasDataKey: true,
    hideTopHandle: true,
}
