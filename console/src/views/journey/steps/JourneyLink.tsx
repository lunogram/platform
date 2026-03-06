import { useCallback } from "react"
import api from "../../../api"
import type { Journey, JourneyStepType } from "../../../types"
import { Combobox } from "@/components/ui/combobox"
import { Label } from "@/components/ui/label"
import { LinkStepIcon } from "../../../components/icons"
import { useResolver } from "../../../hooks"
import { useTranslation } from "react-i18next"
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { ExternalLink } from "lucide-react"
import { Button } from "@/components/ui/button"
import type { UUID } from "@/types/common"
import { NIL } from "uuid"

interface JourneyLinkConfig {
    target_id: UUID
    delay: "1 minute" | "15 minutes" | "1 hour" | "1 day"
}

const delays = ["1 minute", "15 minutes", "1 hour", "1 day"] as const

const delayLabels: Record<(typeof delays)[number], string> = {
    "1 minute": "1 min",
    "15 minutes": "15 min",
    "1 hour": "1 hour",
    "1 day": "1 day",
}

type JourneyOption = Journey & { path: string }

export const journeyLinkStep: JourneyStepType<JourneyLinkConfig> = {
    name: "link",
    icon: <LinkStepIcon />,
    category: "action",
    description: "link_desc",
    Describe({ project, journey, value: { target_id } }) {
        const { t } = useTranslation()
        const [target] = useResolver(
            useCallback(async () => {
                if (target_id === journey.id) {
                    return journey
                }
                if (target_id) {
                    return await api.journeys.get(project.id, target_id)
                }
                return null
            }, [project, journey, target_id]),
        )
        if (target === journey) {
            return (
                <>
                    {t("restart") + " "}
                    <strong>{target.name}</strong>
                </>
            )
        }
        if (target) {
            return (
                <>
                    {t("start_journey")}
                    <strong>{target.name}</strong>
                </>
            )
        }
        return <>{t("link_empty")} &#8211;</>
    },
    newData: async () => ({
        target_id: NIL as UUID,
        delay: "1 day",
    }),
    Edit({ value, onChange, project }) {
        const { t } = useTranslation()
        const [target] = useResolver(
            useCallback(async () => {
                if (value.target_id && value.target_id !== NIL) {
                    return await api.journeys.get(project.id, value.target_id)
                }
                return null
            }, [project.id, value.target_id]),
        )

        const handleSearch = useCallback(
            async (query: string): Promise<JourneyOption[]> => {
                const result = await api.journeys.search(project.id, {
                    search: query,
                    limit: 50,
                })
                return result.results.map((j) => ({ ...j, path: j.id }))
            },
            [project.id],
        )

        return (
            <>
                <div className="space-y-1.5">
                    <Label className="text-sm font-medium">
                        {t("target_journey")}
                        <span className="text-destructive"> *</span>
                    </Label>
                    <p className="text-sm text-muted-foreground">
                        {t("target_journey_desc")}
                    </p>
                    <div className="flex items-center gap-1.5">
                        <Combobox<JourneyOption>
                            onSearch={handleSearch}
                            value={value.target_id === NIL ? "" : value.target_id}
                            displayValue={target?.name}
                            onValueChange={(target_id) =>
                                onChange({
                                    ...value,
                                    target_id: (target_id || NIL) as UUID,
                                })
                            }
                            placeholder={t("target_journey")}
                            className="flex-1"
                            renderOption={(option) => option.name}
                        />
                        {target && (
                            <Button
                                variant="ghost"
                                size="icon"
                                className="h-8 w-8 shrink-0"
                                type="button"
                                onClick={() =>
                                    window.open(
                                        `/projects/${project.id}/journeys/${target.id}`,
                                    )
                                }
                            >
                                <ExternalLink className="h-4 w-4" />
                            </Button>
                        )}
                    </div>
                </div>
                <div className="space-y-1.5">
                    <Label className="text-sm font-medium">{t("delay")}</Label>
                    <Tabs
                        value={value.delay}
                        onValueChange={(delay) =>
                            onChange({
                                ...value,
                                delay: delay as JourneyLinkConfig["delay"],
                            })
                        }
                    >
                        <TabsList className="w-full">
                            {delays.map((key) => (
                                <TabsTrigger key={key} value={key} className="flex-1">
                                    {delayLabels[key]}
                                </TabsTrigger>
                            ))}
                        </TabsList>
                    </Tabs>
                </div>
            </>
        )
    },
    validate: ({ target_id, delay }) => {
        return !!target_id && !!delay
    },
}
