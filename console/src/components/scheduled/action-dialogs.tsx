import { useState } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Label } from "@/components/ui/label"
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from "@/components/ui/dialog"
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select"
import {
    useDeleteScheduled,
    useUpdateScheduled,
    type ScheduledItem,
    type ScheduledSubjectType,
} from "./queries"

export type ScheduledAction = "delete" | "pause" | "resume"

interface ScheduledActionDialogsProps {
    subjectType: ScheduledSubjectType
    subjectId: string
    /** The item and action whose confirmation is open, if any. */
    target: { item: ScheduledItem; action: ScheduledAction } | null
    onClose: () => void
    nameOf: (item: ScheduledItem) => string
}

/** The confirmations for deleting, pausing and resuming a scheduled item. */
export function ScheduledActionDialogs({
    subjectType,
    subjectId,
    target,
    onClose,
    nameOf,
}: ScheduledActionDialogsProps) {
    const { t } = useTranslation()
    const [pauseMode, setPauseMode] = useState<"immediately" | "after_next_interval">("immediately")
    const [resumeMode, setResumeMode] = useState<"immediately" | "at_next_interval">("immediately")

    const remove = useDeleteScheduled(subjectType, subjectId)
    const pause = useUpdateScheduled(
        subjectType,
        subjectId,
        t("failed_to_pause_scheduled", "Failed to pause schedule"),
    )
    const resume = useUpdateScheduled(
        subjectType,
        subjectId,
        t("failed_to_resume_scheduled", "Failed to resume schedule"),
    )

    const openChange = (open: boolean) => {
        if (!open) onClose()
    }

    // Each confirmation closes right away; a failure is reported by the
    // mutation cache and the list keeps the item as it was.
    const confirmDelete = () => {
        if (!target) return
        onClose()
        remove.mutate(target.item.id)
    }

    const confirmPause = () => {
        if (!target) return
        onClose()
        pause.mutate(
            { itemId: target.item.id, patch: { pause: pauseMode } },
            { onSuccess: () => toast.success(t("scheduled_paused", "Schedule paused")) },
        )
    }

    const confirmResume = () => {
        if (!target) return
        onClose()
        resume.mutate(
            { itemId: target.item.id, patch: { resume: resumeMode } },
            { onSuccess: () => toast.success(t("scheduled_resumed", "Schedule resumed")) },
        )
    }

    return (
        <>
            {/* Delete Confirmation Dialog */}
            <Dialog open={target?.action === "delete"} onOpenChange={openChange}>
                <DialogContent>
                    <DialogHeader>
                        <DialogTitle>{t("delete_scheduled", "Delete Schedule")}</DialogTitle>
                        <DialogDescription>
                            {t(
                                "delete_scheduled_confirmation",
                                `Are you sure you want to delete "${target ? nameOf(target.item) : ""}"? This action cannot be undone.`,
                            )}
                        </DialogDescription>
                    </DialogHeader>
                    <DialogFooter>
                        <Button variant="ghost" onClick={onClose}>
                            {t("cancel")}
                        </Button>
                        <Button variant="destructive" onClick={confirmDelete}>
                            {t("delete")}
                        </Button>
                    </DialogFooter>
                </DialogContent>
            </Dialog>

            {/* Pause Dialog */}
            <Dialog open={target?.action === "pause"} onOpenChange={openChange}>
                <DialogContent>
                    <DialogHeader>
                        <DialogTitle>{t("pause_scheduled", "Pause Schedule")}</DialogTitle>
                        <DialogDescription>
                            {t("pause_scheduled_description", "Choose how to pause this schedule.")}
                        </DialogDescription>
                    </DialogHeader>
                    <div className="py-4">
                        <Label className="mb-2 block">{t("pause_mode", "Pause mode")}</Label>
                        <Select
                            value={pauseMode}
                            onValueChange={(v) =>
                                setPauseMode(v as "immediately" | "after_next_interval")
                            }
                        >
                            <SelectTrigger>
                                <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                                <SelectItem value="immediately">
                                    {t("pause_immediately", "Immediately")}
                                </SelectItem>
                                <SelectItem value="after_next_interval">
                                    {t("pause_after_next_interval", "After next interval")}
                                </SelectItem>
                            </SelectContent>
                        </Select>
                        <p className="text-xs text-muted-foreground mt-2">
                            {pauseMode === "immediately"
                                ? t(
                                      "pause_immediately_help",
                                      "Deletes all pending scheduled events right away.",
                                  )
                                : t(
                                      "pause_after_next_interval_help",
                                      "Keeps existing scheduled events but prevents advancement to the next occurrence.",
                                  )}
                        </p>
                    </div>
                    <DialogFooter>
                        <Button variant="ghost" onClick={onClose}>
                            {t("cancel")}
                        </Button>
                        <Button onClick={confirmPause}>{t("pause", "Pause")}</Button>
                    </DialogFooter>
                </DialogContent>
            </Dialog>

            {/* Resume Dialog */}
            <Dialog open={target?.action === "resume"} onOpenChange={openChange}>
                <DialogContent>
                    <DialogHeader>
                        <DialogTitle>{t("resume_scheduled", "Resume Schedule")}</DialogTitle>
                        <DialogDescription>
                            {t(
                                "resume_scheduled_description",
                                "Choose how to resume this schedule.",
                            )}
                        </DialogDescription>
                    </DialogHeader>
                    <div className="py-4">
                        <Label className="mb-2 block">{t("resume_mode", "Resume mode")}</Label>
                        <Select
                            value={resumeMode}
                            onValueChange={(v) =>
                                setResumeMode(v as "immediately" | "at_next_interval")
                            }
                        >
                            <SelectTrigger>
                                <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                                <SelectItem value="immediately">
                                    {t("resume_immediately", "Immediately")}
                                </SelectItem>
                                <SelectItem value="at_next_interval">
                                    {t("resume_at_next_interval", "At next interval")}
                                </SelectItem>
                            </SelectContent>
                        </Select>
                        <p className="text-xs text-muted-foreground mt-2">
                            {resumeMode === "immediately"
                                ? t(
                                      "resume_immediately_help",
                                      "Resumes and recomputes the next occurrence from now.",
                                  )
                                : t(
                                      "resume_at_next_interval_help",
                                      "Resumes and recomputes the next natural interval from the existing anchor.",
                                  )}
                        </p>
                    </div>
                    <DialogFooter>
                        <Button variant="ghost" onClick={onClose}>
                            {t("cancel")}
                        </Button>
                        <Button onClick={confirmResume}>{t("resume", "Resume")}</Button>
                    </DialogFooter>
                </DialogContent>
            </Dialog>
        </>
    )
}
