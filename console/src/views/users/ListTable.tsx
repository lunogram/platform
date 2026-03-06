import type { Key } from "react"
import type { List, ListState, SearchParams, SearchResult } from "../../types"
import { SearchTable, useSearchTableState } from "@/components/search-table"
import { Badge } from "@/components/ui/badge"
import type { BadgeProps } from "@/components/ui/badge"
import { snakeToTitle } from "../../utils"
import { useRoute } from "../router"
import Menu, { MenuItem } from "@/components/menu"
import { ArchiveIcon, DuplicateIcon, EditIcon } from "../../components/icons"
import { oapiClient } from "@/oapi/client"
import { useNavigate, useParams } from "react-router"
import { Translation, useTranslation } from "react-i18next"
import type { UUID } from "@/types/common"
import { NIL } from "uuid"

interface ListTableParams {
    search: (params: SearchParams) => Promise<SearchResult<List>>
    title?: string
    selectedRow?: Key
    onSelectRow?: (list: List) => void
}

export const ListTag = ({ state, progress }: Pick<List, "state" | "progress">) => {
    const variant: Record<ListState, BadgeProps["variant"]> = {
        draft: "secondary",
        loading: "default",
        ready: "outline",
    }

    const complete = progress?.complete ?? 0
    const total = progress?.total ?? 0
    const percent = total > 0 ? complete / total : 0
    const percentStr = percent.toLocaleString(undefined, {
        style: "percent",
        minimumFractionDigits: 0,
    })

    return (
        <Badge variant={variant[state]}>
            <Translation>{(t) => t(state)}</Translation>
            {progress && ` (${percentStr})`}
        </Badge>
    )
}

export default function ListTable({ search, selectedRow, onSelectRow, title }: ListTableParams) {
    const route = useRoute()
    const { t } = useTranslation()
    const navigate = useNavigate()
    const { projectId = NIL as UUID } = useParams<{ projectId: UUID }>()

    function handleOnSelectRow(list: List) {
        if (onSelectRow) {
            onSelectRow(list)
        } else {
            route(`lists/${list.id}`)
        }
    }

    const handleDuplicateList = async (id: UUID) => {
        const res = await oapiClient.POST(`/api/admin/projects/{projectID}/lists/{listID}/duplicate`, {
            params: {
                path: {
                    projectID: projectId,
                    listID: id,
                },
            },
        })
        await navigate(res.data?.id ? `lists/${res.data.id}` : 'lists')
    }

    const handleArchiveList = async (id: UUID) => {
        await oapiClient.DELETE(`/api/admin/projects/{projectID}/lists/{listID}`, {
            params: {
                path: {
                    projectID: projectId,
                    listID: id,
                },
            },
        })
        await state.reload()
    }

    const state = useSearchTableState(search)

    return (
        <SearchTable
            {...state}
            title={title}
            itemKey={({ item }) => item.id}
            columns={[
                {
                    key: "name",
                    title: t("name"),
                    sortable: true,
                    minWidth: "200px",
                },
                {
                    key: "type",
                    title: t("type"),
                    cell: ({ item: { type } }) => snakeToTitle(type),
                    sortable: true,
                },
                {
                    key: "users_count",
                    title: t("users_count"),
                    cell: ({ item }) => item.users_count?.toLocaleString(),
                },
                {
                    key: "state",
                    title: t("state"),
                    cell: ({ item }) => <ListTag state={item.state} progress={item.progress} />,
                },
                {
                    key: "created_at",
                    title: t("created_at"),
                    sortable: true,
                },
                {
                    key: "updated_at",
                    title: t("updated_at"),
                    sortable: true,
                },
                {
                    key: "options",
                    title: t("options"),
                    cell: ({ item }) => (
                        <Menu size="min">
                            <MenuItem onClick={() => handleOnSelectRow(item)}>
                                <EditIcon />
                                {t("edit")}
                            </MenuItem>
                            <MenuItem onClick={async () => await handleDuplicateList(item.id)}>
                                <DuplicateIcon />
                                {t("duplicate")}
                            </MenuItem>
                            <MenuItem onClick={async () => await handleArchiveList(item.id)}>
                                <ArchiveIcon />
                                {t("archive")}
                            </MenuItem>
                        </Menu>
                    ),
                },
            ]}
            selectedRow={selectedRow}
            onSelectRow={(list) => handleOnSelectRow(list)}
            enableSearch
            tagEntity="lists"
        />
    )
}
