import { useState } from "react"
import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { useTranslation } from "react-i18next"
import type { OrganizationFormValues } from "@/validation/organizations/organization-form"
import { organizationFormSchema } from "@/validation/organizations/organization-form"
import type { UpsertOrganization } from "@/oapi/client"
import { useCreateOrganization } from "./queries"

import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from "@/components/ui/dialog"
import { Label } from "@/components/ui/label"
import { AttributeEditor } from "@/components/ui/attribute-editor"

interface CreateOrganizationDialogProps {
    open: boolean
    onOpenChange: (open: boolean) => void
}

export function CreateOrganizationDialog({ open, onOpenChange }: CreateOrganizationDialogProps) {
    const { t } = useTranslation()
    const createMutation = useCreateOrganization()
    const isCreating = createMutation.isPending
    const [newOrgData, setNewOrgData] = useState<Record<string, unknown>>({})

    const orgForm = useForm<OrganizationFormValues>({
        resolver: zodResolver(organizationFormSchema),
        defaultValues: {
            external_id: "",
            name: "",
        },
    })

    // On failure the mutation cache shows the error and the dialog keeps the input.
    const createOrganization = (formData: OrganizationFormValues) =>
        createMutation.mutate(
            {
                identifier: [{ source: "default", external_id: formData.external_id.trim() }],
                name: formData.name?.trim() || undefined,
                data: Object.keys(newOrgData).length > 0 ? newOrgData : undefined,
            } as UpsertOrganization,
            {
                onSuccess: () => {
                    onOpenChange(false)
                    orgForm.reset()
                    setNewOrgData({})
                },
            },
        )

    return (
        <Dialog open={open} onOpenChange={onOpenChange}>
            <DialogContent className="sm:max-w-2xl max-h-[90vh] overflow-y-auto">
                <DialogHeader>
                    <DialogTitle>{t("create_organization")}</DialogTitle>
                    <DialogDescription>
                        {t(
                            "create_organization_description",
                            "Create a new organization to group users together.",
                        )}
                    </DialogDescription>
                </DialogHeader>
                <div className="grid gap-4 py-4">
                    <div className="grid sm:grid-cols-2 gap-4">
                        <div className="grid gap-2 content-start">
                            <Label htmlFor="external_id">{t("identifier", "Identifier")} *</Label>
                            <Input
                                id="external_id"
                                placeholder={t("enter_identifier", "e.g., org-123")}
                                {...orgForm.register("external_id")}
                            />
                            {orgForm.formState.errors.external_id && (
                                <p className="text-sm text-destructive">
                                    {orgForm.formState.errors.external_id.message}
                                </p>
                            )}
                            <p className="text-xs text-muted-foreground">
                                {t("identifier_help", "A unique identifier from your system")}
                            </p>
                        </div>
                        <div className="grid gap-2 content-start">
                            <Label htmlFor="name">{t("name")}</Label>
                            <Input
                                id="name"
                                placeholder={t("enter_organization_name", "e.g., Acme Corp")}
                                {...orgForm.register("name")}
                            />
                        </div>
                    </div>
                    <div className="grid gap-2">
                        <Label>{t("data", "Data")}</Label>
                        <AttributeEditor
                            value={newOrgData}
                            onChange={setNewOrgData}
                            emptyTitle={t("no_data", "No data")}
                            emptyDescription={t(
                                "no_data_description_org",
                                "Add custom attributes to this organization.",
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
                    <Button
                        onClick={orgForm.handleSubmit(createOrganization)}
                        disabled={isCreating}
                    >
                        {isCreating ? t("creating") : t("create")}
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    )
}
