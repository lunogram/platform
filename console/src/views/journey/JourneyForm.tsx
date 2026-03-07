import { useContext, useState } from "react"
import { toast } from "sonner"
import api from "../../api"
import { ProjectContext } from "../../contexts"
import type { Journey } from "../../types"
import { useTranslation } from "react-i18next"
import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"
import { Label } from "@/components/ui/label"
import { Button } from "@/components/ui/button"
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs"

interface JourneyFormProps {
    journey?: Journey
    onSaved?: (journey: Journey) => void
}

export function JourneyForm({ journey, onSaved }: JourneyFormProps) {
    const { t } = useTranslation()
    const [project] = useContext(ProjectContext)
    const [name, setName] = useState(journey?.name ?? "")
    const [description, setDescription] = useState(journey?.description ?? "")
    const [status, setStatus] = useState(journey?.status ?? "draft")
    const [saving, setSaving] = useState(false)

    const isCreated = !!journey?.id
    const isPublished = isCreated && journey?.status !== "draft"

    async function handleSubmit(e: React.FormEvent) {
        e.preventDefault()
        setSaving(true)
        try {
            const saved = journey?.id
                ? await api.journeys.update(project.id, journey.id, {
                      name,
                      description,
                      status,
                      tags: journey.tags,
                  })
                : await api.journeys.create(project.id, {
                      name,
                      description,
                      status,
                      tags: journey?.tags,
                  })
            toast.success(t("journey_saved"))
            onSaved?.(saved)
        } finally {
            setSaving(false)
        }
    }

    return (
        <form onSubmit={handleSubmit} className="space-y-4">
            <div className="space-y-1.5">
                <Label htmlFor="journey-name" className="text-sm font-medium">
                    {t("name")}
                    <span className="text-destructive"> *</span>
                </Label>
                <Input
                    id="journey-name"
                    value={name}
                    onChange={(e) => setName(e.target.value)}
                    required
                />
            </div>
            <div className="space-y-1.5">
                <Label htmlFor="journey-description" className="text-sm font-medium">
                    {t("description")}
                </Label>
                <Textarea
                    id="journey-description"
                    value={description}
                    onChange={(e) => setDescription(e.target.value)}
                    rows={3}
                />
            </div>
            {isPublished && (
                <div className="space-y-1.5">
                    <Label className="text-sm font-medium">
                        {t("status")}
                        <span className="text-destructive"> *</span>
                    </Label>
                    <Tabs
                        value={status === "published" ? "live" : "off"}
                        onValueChange={(v) => setStatus(v === "live" ? "published" : "archived")}
                    >
                        <TabsList className="w-full">
                            <TabsTrigger value="live" className="flex-1">
                                {t("live")}
                            </TabsTrigger>
                            <TabsTrigger value="off" className="flex-1">
                                {t("off")}
                            </TabsTrigger>
                        </TabsList>
                    </Tabs>
                </div>
            )}
            <Button type="submit" className="w-full" isLoading={saving} disabled={!name.trim()}>
                {t("save")}
            </Button>
        </form>
    )
}
