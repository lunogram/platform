import { useState } from "react"
import { useForm, Controller } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"
import { scheduledCreateSchema } from "@/validation/scheduled/scheduled-create"
import type { ScheduledCreateFormValues } from "@/validation/scheduled/scheduled-create"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { AttributeEditor } from "@/components/ui/attribute-editor"
import { Label } from "@/components/ui/label"
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from "@/components/ui/dialog"
import { Combobox } from "@/components/ui/combobox"
import { useCreateScheduled, type ScheduledSubjectType } from "./queries"

/** Says the schedule names failed to load, with a retry. */
export function SchemasLoadError({ onRetry }: { onRetry: () => void }) {
    const { t } = useTranslation()
    return (
        <div className="flex items-center justify-between gap-3 rounded-md border px-3 py-2">
            <span className="text-sm text-destructive">
                {t("scheduled_schemas_load_failed", "Couldn't load schedule names.")}
            </span>
            <Button type="button" variant="outline" size="sm" onClick={onRetry}>
                {t("retry", "Retry")}
            </Button>
        </div>
    )
}

interface CreateScheduledDialogProps {
    open: boolean
    onOpenChange: (open: boolean) => void
    subjectType: ScheduledSubjectType
    subjectId: string
    scheduleOptions: Array<{ path: string }>
    /** Set when the schedule names failed to load; retries loading them. */
    onRetrySchemas?: () => void
}

export function CreateScheduledDialog({
    open,
    onOpenChange,
    subjectType,
    subjectId,
    scheduleOptions,
    onRetrySchemas,
}: CreateScheduledDialogProps) {
    const { t } = useTranslation()
    const [newScheduledData, setNewScheduledData] = useState<Record<string, unknown>>({})
    const create = useCreateScheduled(subjectType, subjectId)

    const scheduledForm = useForm<ScheduledCreateFormValues>({
        resolver: zodResolver(scheduledCreateSchema),
        defaultValues: {
            scheduled_name: "",
            scheduled_at: "",
            start_at: "",
            interval: "",
        },
    })

    // On failure the mutation cache reports the error; the dialog and the
    // user's input stay so they can correct it and try again.
    const createScheduled = (formData: ScheduledCreateFormValues) =>
        create.mutate(
            {
                scheduled_name: formData.scheduled_name.trim(),
                scheduled_at: formData.scheduled_at || undefined,
                start_at: formData.start_at || undefined,
                interval: formData.interval?.trim() || undefined,
                data: Object.keys(newScheduledData).length > 0 ? newScheduledData : undefined,
            },
            {
                onSuccess: () => {
                    onOpenChange(false)
                    scheduledForm.reset()
                    setNewScheduledData({})
                    toast.success(t("scheduled_created", "Scheduled item created"))
                },
            },
        )

    return (
        <Dialog open={open} onOpenChange={onOpenChange}>
            <DialogContent className="sm:max-w-2xl max-h-[90vh] overflow-y-auto">
                <DialogHeader>
                    <DialogTitle>{t("create_scheduled_item", "Create Scheduled Item")}</DialogTitle>
                    <DialogDescription>
                        {t(
                            "create_scheduled_description",
                            "Create a new scheduled item for this subject.",
                        )}
                    </DialogDescription>
                </DialogHeader>
                <div className="grid gap-4 py-4">
                    {onRetrySchemas && <SchemasLoadError onRetry={onRetrySchemas} />}
                    <div className="grid sm:grid-cols-2 gap-4">
                        <div className="grid gap-2 content-start">
                            <Label>{t("scheduled_schema", "Schedule")} *</Label>
                            <Controller
                                control={scheduledForm.control}
                                name="scheduled_name"
                                render={({ field }) => (
                                    <Combobox
                                        options={scheduleOptions}
                                        value={field.value}
                                        onValueChange={field.onChange}
                                        placeholder={t(
                                            "enter_schedule_name",
                                            "Type or select a schedule name...",
                                        )}
                                        emptyText={t(
                                            "no_schedules_found",
                                            "No matching schedules. Type a name to create one.",
                                        )}
                                    />
                                )}
                            />
                            {scheduledForm.formState.errors.scheduled_name && (
                                <p className="text-sm text-destructive">
                                    {scheduledForm.formState.errors.scheduled_name.message}
                                </p>
                            )}
                        </div>
                        <div className="grid gap-2 content-start">
                            <Label htmlFor="interval">{t("interval", "Interval")}</Label>
                            <Input
                                id="interval"
                                placeholder={t("enter_interval", "e.g., 24 hours, 7 days, 1 week")}
                                {...scheduledForm.register("interval")}
                            />
                            <p className="text-xs text-muted-foreground">
                                {t("interval_help", "Makes this a recurring schedule")}
                            </p>
                        </div>
                    </div>
                    <div className="grid sm:grid-cols-2 gap-4">
                        <div className="grid gap-2 content-start">
                            <Label htmlFor="scheduled_at">
                                {t("scheduled_at", "Scheduled At")}
                            </Label>
                            <Controller
                                control={scheduledForm.control}
                                name="scheduled_at"
                                render={({ field }) => (
                                    <Input
                                        id="scheduled_at"
                                        type="datetime-local"
                                        value={field.value || ""}
                                        onChange={(e) =>
                                            field.onChange(
                                                e.target.value
                                                    ? new Date(e.target.value).toISOString()
                                                    : "",
                                            )
                                        }
                                    />
                                )}
                            />
                            <p className="text-xs text-muted-foreground">
                                {t("scheduled_at_help", "Trigger time for single schedules")}
                            </p>
                        </div>
                        <div className="grid gap-2 content-start">
                            <Label htmlFor="start_at">{t("start_at", "Start At")}</Label>
                            <Controller
                                control={scheduledForm.control}
                                name="start_at"
                                render={({ field }) => (
                                    <Input
                                        id="start_at"
                                        type="datetime-local"
                                        value={field.value || ""}
                                        onChange={(e) =>
                                            field.onChange(
                                                e.target.value
                                                    ? new Date(e.target.value).toISOString()
                                                    : "",
                                            )
                                        }
                                    />
                                )}
                            />
                            <p className="text-xs text-muted-foreground">
                                {t("start_at_help", "Start time for recurring schedules")}
                            </p>
                        </div>
                    </div>
                    <div className="grid gap-2">
                        <Label>{t("data", "Data")}</Label>
                        <AttributeEditor
                            value={newScheduledData}
                            onChange={setNewScheduledData}
                            emptyTitle={t("no_data", "No data")}
                            emptyDescription={t(
                                "no_data_description_scheduled",
                                "Add custom data to this scheduled item.",
                            )}
                        />
                    </div>
                </div>
                <DialogFooter>
                    <Button
                        variant="outline"
                        onClick={() => onOpenChange(false)}
                        disabled={create.isPending}
                    >
                        {t("cancel")}
                    </Button>
                    <Button
                        onClick={scheduledForm.handleSubmit(createScheduled)}
                        disabled={create.isPending}
                    >
                        {create.isPending ? t("creating") : t("create")}
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    )
}
