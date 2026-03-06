import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { ProjectContext } from "@/contexts"
import { useCallback, useContext, useState } from "react"

import oapiClient, { type ActionMeta } from "@/oapi/client"
import { useResolver } from "@/hooks"

import { Button } from "@/components/ui/button"
import { ArrowRight, PlusIcon, Webhook, Zap } from "lucide-react"

import {
    Item,
    ItemActions,
    ItemContent,
    ItemDescription,
    ItemGroup,
    ItemMedia,
    ItemTitle,
} from "@/components/ui/item"

import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogHeader,
    DialogTitle,
    DialogTrigger,
} from "@/components/ui/dialog"

const actionIcons: Record<string, { icon: JSX.Element; color: string }> = {
    webhook: {
        icon: <Webhook strokeWidth={2} />,
        color: "bg-yellow-50 text-yellow-600",
    },
}

const defaultActionIcon = {
    icon: <Zap strokeWidth={2} />,
    color: "bg-blue-50 text-blue-600",
}

interface CreateActionProps {
    open?: boolean
}

export function CreateAction({ open = false }: CreateActionProps) {
    const [project] = useContext(ProjectContext)
    const navigate = useNavigate()
    const { t } = useTranslation()
    const [isOpen, setIsOpen] = useState(open)

    const [actionMetas] = useResolver(
        useCallback(async () => {
            const { data } = await oapiClient.GET("/api/admin/projects/{projectID}/actions/meta", {
                params: { path: { projectID: project.id } },
            })
            return data ?? null
        }, [project.id]),
    )

    function selectType(type: string) {
        setIsOpen(false)
        navigate(`/projects/${project.id}/actions/new/${type}`)
    }

    return (
        <Dialog open={isOpen} onOpenChange={() => setIsOpen(!isOpen)}>
            <DialogTrigger>
                <Button size="lg">
                    <PlusIcon /> {t("create_action", "Create Action")}
                </Button>
            </DialogTrigger>
            <DialogContent>
                <DialogHeader>
                    <DialogTitle>{t("action.create.title", "Create Action")}</DialogTitle>
                    <DialogDescription>
                        {t(
                            "action.create.description",
                            "Select the type of action you want to create.",
                        )}
                    </DialogDescription>
                </DialogHeader>

                <ItemGroup className="gap-2">
                    {actionMetas?.map((meta: ActionMeta) => {
                        const { icon, color } = actionIcons[meta.type] ?? defaultActionIcon
                        return (
                            <Item
                                key={meta.type}
                                variant="outline"
                                className="items-center"
                                asChild
                            >
                                <button
                                    type="button"
                                    className="no-underline cursor-pointer"
                                    onClick={() => selectType(meta.type)}
                                >
                                    <ItemMedia variant="icon" className={color}>
                                        {icon}
                                    </ItemMedia>
                                    <ItemContent>
                                        <ItemTitle>{meta.name}</ItemTitle>
                                        {meta.description && (
                                            <ItemDescription>{meta.description}</ItemDescription>
                                        )}
                                    </ItemContent>
                                    <ItemActions>
                                        <ArrowRight strokeWidth={1} />
                                    </ItemActions>
                                </button>
                            </Item>
                        )
                    })}
                </ItemGroup>
            </DialogContent>
        </Dialog>
    )
}
