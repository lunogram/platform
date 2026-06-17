import { useContext } from "react"
import { Controller, useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { oapiClient } from "@/oapi/client"
import type { components } from "@/oapi/management.generated"
import { ProjectContext } from "../../contexts"
import type { List } from "../../types"
import { useTranslation } from "react-i18next"
import { createWrapperRule } from "./rules/RuleHelpers"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Button } from "@/components/ui/button"
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs"
import {
    listCreateFormSchema,
    type ListCreateFormValues,
} from "@/validation/users/list-create-form"

interface ListCreateFormProps {
    onCreated?: (list: List) => void
}

export function ListCreateForm({ onCreated }: ListCreateFormProps) {
    const { t } = useTranslation()
    const [project] = useContext(ProjectContext)

    const form = useForm<ListCreateFormValues>({
        resolver: zodResolver(listCreateFormSchema),
        defaultValues: {
            name: "",
            type: "dynamic",
        },
    })

    const handleSubmit = form.handleSubmit(async (data) => {
        const rule = createWrapperRule()
        const body: components["schemas"]["CreateList"] = {
            name: data.name,
            type: data.type,
            rule: data.type === "dynamic" ? rule : undefined,
        }
        const { data: created } = await oapiClient.POST("/api/admin/projects/{projectID}/lists", {
            params: { path: { projectID: project.id } },
            // is_visible is not part of the generated CreateList schema but is
            // accepted by the API; include it on the wire while keeping the
            // typed body for the known fields.
            body: { ...body, is_visible: true } as typeof body,
        })
        if (created) onCreated?.(created as List)
    })

    return (
        <form onSubmit={handleSubmit} className="space-y-4">
            <div className="space-y-1.5">
                <Label htmlFor="list-name" className="text-sm font-medium">
                    {t("list_save")}
                    <span className="text-destructive"> *</span>
                </Label>
                <Input id="list-name" {...form.register("name")} required />
                {form.formState.errors.name && (
                    <p className="text-sm text-destructive">{form.formState.errors.name.message}</p>
                )}
            </div>
            <div className="space-y-1.5">
                <Label className="text-sm font-medium">{t("type")}</Label>
                <Controller
                    control={form.control}
                    name="type"
                    render={({ field }) => (
                        <Tabs value={field.value} onValueChange={(v) => field.onChange(v)}>
                            <TabsList className="w-full">
                                <TabsTrigger value="dynamic" className="flex-1 cursor-pointer">
                                    {t("dynamic")}
                                </TabsTrigger>
                                <TabsTrigger value="static" className="flex-1 cursor-pointer">
                                    {t("static")}
                                </TabsTrigger>
                            </TabsList>
                        </Tabs>
                    )}
                />
            </div>
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
