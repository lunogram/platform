/* eslint-disable react-refresh/only-export-components */
import type {
    JourneyStepType,
    Rule,
    RulePath,
    ScheduledSchema,
    ScheduleOffset,
} from "../../../types"
import { EntranceStepIcon } from "../../../components/icons"
import RuleBuilder from "../../users/rules/RuleBuilder"
import { useCallback, useContext, useEffect, useMemo, useRef, useState } from "react"
import { PreferencesContext } from "@/contexts/PreferencesContext"
import { Switch } from "@/components/ui/switch"
import { Label } from "@/components/ui/label"
import { Combobox } from "@/components/ui/combobox"
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs"
import api from "../../../api"
import { Button } from "@/components/ui/button"
import { env } from "../../../config/env"
import { useTranslation, Trans } from "react-i18next"
import { createSimpleEventRule, isEventWrapper } from "../../users/rules/RuleHelpers"
import { ruleDescription } from "../../users/rules/RuleDescriptions"
import { Badge } from "@/components/ui/badge"
import { Webhook, Zap, Copy, Check, CalendarClock } from "lucide-react"
import { ScheduleOffsetCombobox } from "@/components/schedule-offset-combobox"
import { formatOffset } from "@/utils"

import { Link } from "react-router"
import type { UUID } from "@/types/common"

interface EntranceConfig {
    trigger: "none" | "event" | "scheduled"

    // event based
    event_name?: string
    rule?: Rule
    multiple?: boolean
    concurrent?: boolean

    // scheduled based
    scheduled_name?: string
    schedule_offset_id?: UUID
    schedule_offset?: string
    schedule_offset_direction?: string

    references?: Array<{ id: UUID; name: string }>
}

const triggers = ["none", "event", "scheduled"] as const

const triggerConfig = {
    none: { icon: Webhook, label: "API" },
    event: { icon: Zap, label: "Event" },
    scheduled: { icon: CalendarClock, label: "Scheduled" },
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

import type { MutableRefObject } from "react"

interface ScheduledOffsetSelectorProps {
    projectId: string
    scheduledCacheRef: MutableRefObject<ScheduledSchema[] | null>
    scheduledName: string
    value: EntranceConfig
    onChange: (value: EntranceConfig) => void
    onOffsetsChange: (offsets: ScheduleOffset[], schedule: ScheduledSchema) => void
    t: (key: string, fallback?: string) => string
}

function ScheduledOffsetSelector({
    projectId,
    scheduledCacheRef,
    scheduledName,
    value,
    onChange,
    onOffsetsChange,
    t,
}: ScheduledOffsetSelectorProps) {
    const schedule = scheduledCacheRef.current?.find((s) => s.name === scheduledName)
    const offsets = useMemo(() => schedule?.offsets ?? [], [schedule?.offsets])

    // Auto-select the first offset when offsets become available but none is selected.
    // Use a ref for onChange/value to avoid re-triggering when the parent re-renders.
    const onChangeRef = useRef(onChange)
    const valueRef = useRef(value)
    onChangeRef.current = onChange
    valueRef.current = value

    useEffect(() => {
        if (!valueRef.current.schedule_offset_id && offsets.length > 0) {
            const first = offsets[0]
            onChangeRef.current({
                ...valueRef.current,
                schedule_offset_id: first.id,
                schedule_offset: first.offset,
                schedule_offset_direction: first.direction,
            })
        }
    }, [offsets])

    return (
        <div className="space-y-1.5">
            <Label className="inline-flex items-center gap-0.5 text-sm font-medium">
                {t("schedule_offset", "Schedule Offset")}
                <span className="text-destructive">*</span>
            </Label>
            <ScheduleOffsetCombobox
                projectId={projectId}
                scheduledId={schedule?.id ?? ("" as UUID)}
                offsets={offsets}
                value={value.schedule_offset_id}
                onChange={(offsetId, offset, direction) => {
                    onChange({
                        ...value,
                        schedule_offset_id: offsetId,
                        schedule_offset: offset,
                        schedule_offset_direction: direction,
                    })
                }}
                onOffsetsChange={(updated) => {
                    if (schedule) {
                        onOffsetsChange(updated, schedule)
                    }
                }}
                disabled={!schedule}
            />
        </div>
    )
}

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
        trigger: "event",
    }),
    Describe({
        value: {
            trigger,
            event_name,
            rule,
            scheduled_name,
            schedule_offset_id,
            schedule_offset,
            schedule_offset_direction,
            references = [],
        },
    }) {
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

        if (trigger === "scheduled") {
            return (
                <div className="flex items-center gap-1.5 text-sm">
                    <CalendarClock className="size-4 shrink-0 text-muted-foreground" />
                    <span>
                        <span className="font-medium">{scheduled_name}</span>
                        {schedule_offset_id && schedule_offset != null && (
                            <span className="text-muted-foreground">
                                {" "}
                                {formatOffset(
                                    schedule_offset,
                                    schedule_offset_direction ?? "before",
                                )}
                            </span>
                        )}
                    </span>
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

        const cacheRef = useRef<RulePath[] | null>(null)
        const scheduledCacheRef = useRef<ScheduledSchema[] | null>(null)
        const [scheduledCacheVersion, setScheduledCacheVersion] = useState(0)

        const ensureSuggestionsLoaded = useCallback(async () => {
            if (cacheRef.current && scheduledCacheRef.current) return
            const suggestions = await api.projects.pathSuggestions(projectId)
            if (!cacheRef.current) {
                cacheRef.current = Array.isArray(suggestions?.eventPaths)
                    ? suggestions.eventPaths.map((event, index) => ({
                          id: `event-${index}` as UUID,
                          name: event.name,
                          path: event.name,
                          type: "event" as const,
                          data_type: "string" as const,
                          visibility: "public" as const,
                      }))
                    : []
            }
            if (!scheduledCacheRef.current) {
                scheduledCacheRef.current = Array.isArray(suggestions?.scheduledPaths)
                    ? suggestions.scheduledPaths
                    : []
            }
        }, [projectId])

        // Eagerly load suggestions on mount when trigger is scheduled
        // so that the offset selector has data immediately (not just on search)
        useEffect(() => {
            if (value.trigger === "scheduled" && value.scheduled_name) {
                ensureSuggestionsLoaded().then(() => {
                    setScheduledCacheVersion((v) => v + 1)
                })
            }
        }, []) // eslint-disable-line react-hooks/exhaustive-deps

        const handleSearch = useCallback(
            async (query: string): Promise<RulePath[]> => {
                await ensureSuggestionsLoaded()
                if (!query) return cacheRef.current!
                const lower = query.toLowerCase()
                return cacheRef.current!.filter((o) => o.name.toLowerCase().includes(lower))
            },
            [ensureSuggestionsLoaded],
        )

        const handleScheduledSearch = useCallback(
            async (query: string): Promise<RulePath[]> => {
                await ensureSuggestionsLoaded()
                const schedules = scheduledCacheRef.current ?? []
                const paths: RulePath[] = schedules.map((s, index) => ({
                    id: `scheduled-${index}` as UUID,
                    name: s.name,
                    path: s.name,
                    type: "scheduled" as const,
                    data_type: "string" as const,
                    visibility: "public" as const,
                }))
                if (!query) return paths
                const lower = query.toLowerCase()
                return paths.filter((o) => o.name.toLowerCase().includes(lower))
            },
            [ensureSuggestionsLoaded],
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
                            <Combobox<RulePath>
                                onSearch={handleSearch}
                                value={value.event_name ?? ""}
                                displayValue={value.event_name}
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
                                placeholder={t("event_name")}
                                renderOption={(option) => option.name}
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
                {value.trigger === "scheduled" && (
                    <>
                        {/* Schedule name selector */}
                        <div className="space-y-1.5">
                            <Label className="inline-flex items-center gap-0.5 text-sm font-medium">
                                {t("scheduled_name", "Scheduled Name")}
                                <span className="text-destructive">*</span>
                            </Label>
                            <Combobox<RulePath>
                                onSearch={handleScheduledSearch}
                                value={value.scheduled_name ?? ""}
                                displayValue={value.scheduled_name}
                                onValueChange={(scheduled_name) => {
                                    // When schedule changes, reset offset selection
                                    onChange({
                                        ...value,
                                        scheduled_name,
                                        schedule_offset_id: undefined,
                                        event_name: scheduled_name
                                            ? `scheduled.${scheduled_name}`
                                            : undefined,
                                    })
                                }}
                                placeholder={t("scheduled_name", "Scheduled Name")}
                                renderOption={(option) => option.name}
                            />
                        </div>

                        {/* Offset selector - shown after schedule is selected */}
                        {value.scheduled_name && (
                            <ScheduledOffsetSelector
                                key={scheduledCacheVersion}
                                projectId={projectId}
                                scheduledCacheRef={scheduledCacheRef}
                                scheduledName={value.scheduled_name}
                                value={value}
                                onChange={onChange}
                                onOffsetsChange={(updated, schedule) => {
                                    const idx = scheduledCacheRef.current?.findIndex(
                                        (s) => s.id === schedule.id,
                                    )
                                    if (scheduledCacheRef.current && idx != null && idx >= 0) {
                                        scheduledCacheRef.current = scheduledCacheRef.current.map(
                                            (s, i) => (i === idx ? { ...s, offsets: updated } : s),
                                        )
                                        setScheduledCacheVersion((v) => v + 1)
                                    }
                                }}
                                t={t}
                            />
                        )}

                        {/* RuleBuilder - shown after schedule + offset are selected */}
                        {value.scheduled_name && value.schedule_offset_id && value.event_name && (
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

                        {/* Multiple entries toggle */}
                        <div className="flex items-center justify-between gap-4 rounded-lg border p-4">
                            <div className="space-y-0.5">
                                <Label htmlFor="multiple-scheduled" className="text-sm font-medium">
                                    {t("entrance_multiple_entries")}
                                </Label>
                                <p className="text-xs text-muted-foreground">
                                    {t("entrance_multiple_entries_desc")}
                                </p>
                            </div>
                            <Switch
                                id="multiple-scheduled"
                                checked={Boolean(value.multiple)}
                                onCheckedChange={(multiple) => onChange({ ...value, multiple })}
                            />
                        </div>

                        {/* Concurrent entries toggle */}
                        {value.multiple && (
                            <div className="flex items-center justify-between gap-4 rounded-lg border p-4">
                                <div className="space-y-0.5">
                                    <Label
                                        htmlFor="concurrent-scheduled"
                                        className="text-sm font-medium"
                                    >
                                        {t("entrance_simultaneous_entries")}
                                    </Label>
                                    <p className="text-xs text-muted-foreground">
                                        {t("entrance_simultaneous_entries_desc")}
                                    </p>
                                </div>
                                <Switch
                                    id="concurrent-scheduled"
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
    validate: ({ trigger, event_name, scheduled_name, schedule_offset_id }) => {
        if (trigger === "event") {
            return !!event_name
        }
        if (trigger === "scheduled") {
            return !!scheduled_name && !!schedule_offset_id
        }
        return true
    },
    hasDataKey: true,
    hideTopHandle: true,
}
