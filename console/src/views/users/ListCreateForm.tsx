import { useContext, useState } from "react"
import api from "../../api"
import { ProjectContext } from "../../contexts"
import type { List } from "../../types"
import { useTranslation } from "react-i18next"
import { createWrapperRule } from "./rules/RuleHelpers"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Button } from "@/components/ui/button"
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs"

interface ListCreateFormProps {
    onCreated?: (list: List) => void
}

export function ListCreateForm({ onCreated }: ListCreateFormProps) {
    const { t } = useTranslation()
    const [project] = useContext(ProjectContext)
    const [name, setName] = useState("")
    const [type, setType] = useState<"dynamic" | "static">("dynamic")
    const [saving, setSaving] = useState(false)

    async function handleSubmit(e: React.FormEvent) {
        e.preventDefault()
        setSaving(true)
        try {
            const rule = createWrapperRule()
            const created = await api.lists.create(project.id, {
                name,
                type,
                rule: type === "dynamic" ? rule : undefined,
                is_visible: true,
            })
            onCreated?.(created)
        } finally {
            setSaving(false)
        }
    }

    return (
        <form onSubmit={handleSubmit} className="space-y-4">
            <div className="space-y-1.5">
                <Label htmlFor="list-name" className="text-sm font-medium">
                    {t("list_save")}
                    <span className="text-destructive"> *</span>
                </Label>
                <Input
                    id="list-name"
                    value={name}
                    onChange={(e) => setName(e.target.value)}
                    required
                />
            </div>
            <div className="space-y-1.5">
                <Label className="text-sm font-medium">{t("type")}</Label>
                <Tabs value={type} onValueChange={(v) => setType(v as "dynamic" | "static")}>
                    <TabsList className="w-full">
                        <TabsTrigger value="dynamic" className="flex-1">
                            {t("dynamic")}
                        </TabsTrigger>
                        <TabsTrigger value="static" className="flex-1">
                            {t("static")}
                        </TabsTrigger>
                    </TabsList>
                </Tabs>
            </div>
            <Button type="submit" className="w-full" isLoading={saving} disabled={!name.trim()}>
                {t("save")}
            </Button>
        </form>
    )
}
