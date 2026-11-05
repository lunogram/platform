import { useContext, useState } from "react"
import { useTranslation } from "react-i18next"
import { Info } from "lucide-react"
import { toast } from "sonner"
import { ProjectContext } from "@/contexts"
import { DEFAULT_TIME_INPUT_VALUE, toIsoFromDateAndTime } from "@/lib/date-time"
import { Button } from "@/components/ui/button"
import { SenderIdentityCombobox } from "@/components/sender-identity-combobox"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Textarea } from "@/components/ui/textarea"
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
import { useCreateInboxMessage, type InboxChannel, type InboxSubjectType } from "./queries"

interface CreateInboxMessageDialogProps {
    open: boolean
    onOpenChange: (open: boolean) => void
    subjectType: InboxSubjectType
    subjectId: string
}

export function CreateInboxMessageDialog({
    open,
    onOpenChange,
    subjectType,
    subjectId,
}: CreateInboxMessageDialogProps) {
    const { t } = useTranslation()
    const [project] = useContext(ProjectContext)
    const create = useCreateInboxMessage(subjectType, subjectId)
    const isCreating = create.isPending
    const [channel, setChannel] = useState<InboxChannel>("inbox")
    const [senderIdentityId, setSenderIdentityId] = useState("")
    const [title, setTitle] = useState("")
    const [body, setBody] = useState("")
    const [tags, setTags] = useState("")
    const [scheduledDate, setScheduledDate] = useState("")
    const [scheduledTime, setScheduledTime] = useState(DEFAULT_TIME_INPUT_VALUE)

    const resetForm = () => {
        setChannel("inbox")
        setSenderIdentityId("")
        setTitle("")
        setBody("")
        setTags("")
        setScheduledDate("")
        setScheduledTime(DEFAULT_TIME_INPUT_VALUE)
    }

    const handleScheduledDateChange = (nextDate: string) => {
        setScheduledDate(nextDate)
        if (!nextDate) {
            setScheduledTime(DEFAULT_TIME_INPUT_VALUE)
        }
    }

    const createMessage = () => {
        if (channel === "sms" ? !body.trim() : !title.trim()) return
        if ((channel === "email" || channel === "sms") && !senderIdentityId) return

        const dedupedTags = Array.from(
            new Set(
                tags
                    .split(",")
                    .map((tag) => tag.trim())
                    .filter(Boolean),
            ),
        )

        const payload = {
            channel,
            sender_identity_id:
                channel === "push" || channel === "inbox" ? undefined : senderIdentityId,
            content:
                channel === "sms"
                    ? { body: body.trim() }
                    : {
                          title: title.trim(),
                          body: body.trim() || undefined,
                      },
            tags: dedupedTags,
            priority: 3,
            scheduled_at: toIsoFromDateAndTime(scheduledDate, scheduledTime),
        }

        // On failure the mutation cache reports the error; the dialog and
        // the user's input stay so they can try again.
        create.mutate(payload, {
            onSuccess: () => {
                toast.success(t("inbox_message_created", "Inbox message created"))
                resetForm()
                onOpenChange(false)
            },
        })
    }

    return (
        <Dialog
            open={open}
            onOpenChange={(open) => {
                onOpenChange(open)
                if (!open) resetForm()
            }}
        >
            <DialogContent className="sm:max-w-2xl max-h-[90vh] overflow-y-auto">
                <DialogHeader>
                    <DialogTitle>{t("new_inbox_message", "New inbox message")}</DialogTitle>
                    <DialogDescription>
                        {t(
                            "new_inbox_message_description",
                            "Create a message that appears in this subject inbox.",
                        )}
                    </DialogDescription>
                </DialogHeader>
                <div className="grid gap-4 py-4">
                    <div className="grid sm:grid-cols-2 gap-4">
                        <div className="grid gap-2 content-start">
                            <Label htmlFor="inbox-channel">{t("channel", "Channel")} *</Label>
                            <Select
                                value={channel}
                                onValueChange={(value) => {
                                    setChannel(value as InboxChannel)
                                    setSenderIdentityId("")
                                }}
                            >
                                <SelectTrigger id="inbox-channel">
                                    <SelectValue />
                                </SelectTrigger>
                                <SelectContent>
                                    <SelectItem value="inbox">{t("inbox", "Inbox")}</SelectItem>
                                    <SelectItem value="email">{t("email", "Email")}</SelectItem>
                                    <SelectItem value="sms">{t("sms", "SMS")}</SelectItem>
                                    <SelectItem value="push">{t("push", "Push")}</SelectItem>
                                </SelectContent>
                            </Select>
                            <p className="text-xs text-muted-foreground">
                                {t(
                                    "inbox_channel_help",
                                    "Delivery channel the message represents.",
                                )}
                            </p>
                        </div>
                        <div className="grid gap-2 content-start">
                            {channel === "push" || channel === "inbox" ? (
                                <>
                                    <Label>{t("sender", "Sender")}</Label>
                                    <div className="flex items-start gap-2 rounded-md border bg-muted/40 px-3 py-2 text-sm text-muted-foreground">
                                        <Info
                                            aria-hidden="true"
                                            className="mt-0.5 h-4 w-4 shrink-0"
                                        />
                                        <span>
                                            {channel === "inbox"
                                                ? t(
                                                      "inbox_no_sender_needed",
                                                      "Inbox messages do not require a sender identity.",
                                                  )
                                                : t(
                                                      "push_uses_project_settings",
                                                      "Push messages use the project's push provider settings.",
                                                  )}
                                        </span>
                                    </div>
                                </>
                            ) : (
                                <>
                                    <Label htmlFor="inbox-sender-identity">
                                        {channel === "email"
                                            ? t("from_address", "From address")
                                            : t("from_number", "From number")}{" "}
                                        *
                                    </Label>
                                    <SenderIdentityCombobox
                                        projectId={project.id}
                                        channel={channel === "email" ? "email" : "sms"}
                                        value={senderIdentityId}
                                        onChange={setSenderIdentityId}
                                        placeholder={
                                            channel === "email"
                                                ? t("select_from_address", "Select from address...")
                                                : t("select_from_number", "Select from number...")
                                        }
                                    />
                                    <p className="text-xs text-muted-foreground">
                                        {channel === "email"
                                            ? t(
                                                  "inbox_from_address_help",
                                                  "Verified sender to use as the from address.",
                                              )
                                            : t(
                                                  "inbox_from_number_help",
                                                  "Verified sender number to use.",
                                              )}
                                    </p>
                                </>
                            )}
                        </div>
                    </div>

                    {channel !== "sms" && (
                        <div className="grid gap-2">
                            <Label htmlFor="inbox-title">
                                {channel === "email"
                                    ? t("subject", "Subject")
                                    : t("title", "Title")}{" "}
                                *
                            </Label>
                            <Input
                                id="inbox-title"
                                value={title}
                                onChange={(event) => setTitle(event.target.value)}
                                placeholder={
                                    channel === "email"
                                        ? t("inbox_subject_placeholder", "Message subject")
                                        : t("inbox_title_placeholder", "Message title")
                                }
                            />
                        </div>
                    )}

                    <div className="grid gap-2">
                        <Label htmlFor="inbox-body">
                            {t("body", "Body")}
                            {channel === "sms" ? " *" : ""}
                        </Label>
                        <Textarea
                            id="inbox-body"
                            value={body}
                            onChange={(event) => setBody(event.target.value)}
                            rows={4}
                            placeholder={t(
                                "inbox_body_placeholder",
                                "Message body shown to the recipient",
                            )}
                        />
                    </div>

                    <div className="grid sm:grid-cols-2 gap-4">
                        <div className="grid gap-2 content-start">
                            <Label htmlFor="inbox-tags">{t("tags", "Tags")}</Label>
                            <Input
                                id="inbox-tags"
                                value={tags}
                                onChange={(event) => setTags(event.target.value)}
                                placeholder={t("inbox_tags_placeholder", "billing, onboarding")}
                            />
                            <p className="text-xs text-muted-foreground">
                                {t("inbox_tags_help", "Comma separated. Duplicates are removed.")}
                            </p>
                        </div>
                        <div className="grid gap-2 content-start">
                            <Label htmlFor="inbox-scheduled-at">
                                {t("scheduled_at", "Scheduled at")}
                            </Label>
                            <div className="flex gap-2">
                                <Input
                                    id="inbox-scheduled-at"
                                    type="date"
                                    value={scheduledDate}
                                    onChange={(event) =>
                                        handleScheduledDateChange(event.target.value)
                                    }
                                />
                                <Input
                                    id="inbox-scheduled-time"
                                    type="time"
                                    value={scheduledTime}
                                    onChange={(event) => setScheduledTime(event.target.value)}
                                    disabled={!scheduledDate}
                                />
                            </div>
                            <p className="text-xs text-muted-foreground">
                                {t(
                                    "inbox_scheduled_at_hint",
                                    "Leave empty to publish the inbox message immediately.",
                                )}
                            </p>
                        </div>
                    </div>
                </div>
                <DialogFooter>
                    <Button
                        variant="ghost"
                        onClick={() => {
                            onOpenChange(false)
                            resetForm()
                        }}
                        disabled={isCreating}
                    >
                        {t("cancel", "Cancel")}
                    </Button>
                    <Button
                        onClick={createMessage}
                        disabled={
                            isCreating ||
                            (channel === "sms" ? !body.trim() : !title.trim()) ||
                            ((channel === "email" || channel === "sms") && !senderIdentityId)
                        }
                    >
                        {isCreating ? t("creating", "Creating...") : t("create", "Create")}
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    )
}
