import { useContext, useMemo, useState } from "react"
import { useForm, Controller } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { useTranslation } from "react-i18next"
import { useQuery } from "@tanstack/react-query"
import { toast } from "sonner"
import type { EventFormValues } from "@/validation/event-form"
import { eventFormSchema } from "@/validation/event-form"
import { ProjectContext } from "../../contexts"
import { useCreateUserEvent, userEventSchemasQuery } from "./queries"

import { Button } from "@/components/ui/button"
import { Combobox } from "@/components/ui/combobox"
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

interface CreateUserEventDialogProps {
    open: boolean
    onOpenChange: (open: boolean) => void
}

/** Sends an event for the user in context. */
export function CreateUserEventDialog({ open, onOpenChange }: CreateUserEventDialogProps) {
    const { t } = useTranslation()
    const [project] = useContext(ProjectContext)
    const createEvent = useCreateUserEvent()
    const isCreating = createEvent.isPending
    const [newEventData, setNewEventData] = useState<Record<string, unknown>>({})

    const eventForm = useForm<EventFormValues>({
        resolver: zodResolver(eventFormSchema),
        defaultValues: { event_name: "" },
    })

    // Only suggestions: without them the name can still be typed.
    const schemas = useQuery(userEventSchemasQuery(project.id))
    const eventOptions = useMemo(
        () => (schemas.data ?? []).map((s) => ({ path: s.name })),
        [schemas.data],
    )

    const submit = (formData: EventFormValues) => {
        createEvent.mutate(
            {
                name: formData.event_name.trim(),
                data: Object.keys(newEventData).length > 0 ? newEventData : undefined,
            },
            {
                onSuccess: () => {
                    onOpenChange(false)
                    eventForm.reset()
                    setNewEventData({})
                    toast.success(t("event_created", "Event created"))
                },
            },
        )
    }

    return (
        <Dialog open={open} onOpenChange={onOpenChange}>
            <DialogContent className="sm:max-w-2xl max-h-[90vh] overflow-y-auto">
                <DialogHeader>
                    <DialogTitle>{t("create_event", "Create Event")}</DialogTitle>
                    <DialogDescription>
                        {t("create_event_description", "Create a new event for this user.")}
                    </DialogDescription>
                </DialogHeader>
                <div className="grid gap-4 py-4">
                    <div className="grid gap-2">
                        <Label>{t("event_name", "Event")} *</Label>
                        <Controller
                            control={eventForm.control}
                            name="event_name"
                            render={({ field }) => (
                                <Combobox
                                    options={eventOptions}
                                    value={field.value}
                                    onValueChange={field.onChange}
                                    placeholder={t(
                                        "enter_event_name",
                                        "Type or select an event name...",
                                    )}
                                    emptyText={t(
                                        "no_events_found",
                                        "No matching events. Type a name to create one.",
                                    )}
                                />
                            )}
                        />
                        {eventForm.formState.errors.event_name && (
                            <p className="text-sm text-destructive">
                                {eventForm.formState.errors.event_name.message}
                            </p>
                        )}
                    </div>
                    <div className="grid gap-2">
                        <Label>{t("data", "Data")}</Label>
                        <AttributeEditor
                            value={newEventData}
                            onChange={setNewEventData}
                            emptyTitle={t("no_data", "No data")}
                            emptyDescription={t(
                                "no_data_description_event",
                                "Add custom data to this event.",
                            )}
                        />
                    </div>
                </div>
                <DialogFooter>
                    <Button
                        variant="outline"
                        onClick={() => onOpenChange(false)}
                        disabled={isCreating}
                    >
                        {t("cancel")}
                    </Button>
                    <Button onClick={eventForm.handleSubmit(submit)} disabled={isCreating}>
                        {isCreating ? t("creating") : t("create")}
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    )
}
