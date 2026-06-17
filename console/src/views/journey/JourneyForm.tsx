import { useContext } from "react"
import { Controller, useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { toast } from "sonner"
import { oapiClient } from "@/oapi/client"
import type { components } from "@/oapi/management.generated"
import { ProjectContext } from "../../contexts"
import type { Journey } from "../../types"
import { useTranslation } from "react-i18next"
import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"
import { Label } from "@/components/ui/label"
import { Button } from "@/components/ui/button"
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { journeyFormSchema, type JourneyFormValues } from "@/validation/journey/journey-form"

interface JourneyFormProps {
    journey?: Journey
    onSaved?: (journey: Journey) => void
}

export function JourneyForm({ journey, onSaved }: JourneyFormProps) {
    const { t } = useTranslation()
    const [project] = useContext(ProjectContext)

    const form = useForm<JourneyFormValues>({
        resolver: zodResolver(journeyFormSchema),
        defaultValues: {
            name: journey?.name ?? "",
            description: journey?.description ?? "",
            status: journey?.status ?? "draft",
        },
    })

    const isCreated = !!journey?.id
    const isPublished = isCreated && journey?.status !== "draft"

    const handleSubmit = form.handleSubmit(async (data) => {
        try {
            const { data: saved } = journey?.id
                ? await oapiClient.PATCH("/api/admin/projects/{projectID}/journeys/{journeyID}", {
                      params: {
                          path: { projectID: project.id, journeyID: journey.id },
                      },
                      // status/tags are accepted by the backend but not yet in
                      // the OpenAPI spec; cast to the documented schema shape.
                      body: {
                          name: data.name,
                          description: data.description,
                          status: data.status,
                          tags: journey.tags,
                      } as components["schemas"]["UpdateJourney"],
                  })
                : await oapiClient.POST("/api/admin/projects/{projectID}/journeys", {
                      params: { path: { projectID: project.id } },
                      body: {
                          name: data.name,
                          description: data.description,
                          status: data.status,
                          tags: journey?.tags,
                      } as components["schemas"]["CreateJourney"],
                  })
            if (!saved) return
            toast.success(t("journey_saved"))
            onSaved?.(saved as Journey)
        } catch {
            // form handleSubmit catches errors
        }
    })

    return (
        <form onSubmit={handleSubmit} className="space-y-4">
            <div className="space-y-1.5">
                <Label htmlFor="journey-name" className="text-sm font-medium">
                    {t("name")}
                    <span className="text-destructive"> *</span>
                </Label>
                <Input id="journey-name" {...form.register("name")} />
                {form.formState.errors.name && (
                    <p className="text-sm text-destructive">{form.formState.errors.name.message}</p>
                )}
            </div>
            <div className="space-y-1.5">
                <Label htmlFor="journey-description" className="text-sm font-medium">
                    {t("description")}
                </Label>
                <Textarea id="journey-description" {...form.register("description")} rows={3} />
            </div>
            {isPublished && (
                <div className="space-y-1.5">
                    <Label className="text-sm font-medium">
                        {t("status")}
                        <span className="text-destructive"> *</span>
                    </Label>
                    <Controller
                        control={form.control}
                        name="status"
                        render={({ field }) => (
                            <Tabs
                                value={field.value === "published" ? "live" : "off"}
                                onValueChange={(v) =>
                                    field.onChange(v === "live" ? "published" : "archived")
                                }
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
                        )}
                    />
                </div>
            )}
            <Button
                type="submit"
                className="w-full"
                isLoading={form.formState.isSubmitting}
                disabled={form.formState.isSubmitting}
            >
                {t("save")}
            </Button>
        </form>
    )
}
