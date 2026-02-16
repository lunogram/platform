import { useCallback, useContext, useState } from 'react'
import { useNavigate } from 'react-router'
import api from '../../api'
import { Button } from '@/components/ui/button'
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogHeader,
    DialogTitle,
    DialogTrigger,
} from '@/components/ui/dialog'
import PageContent from '../../ui/PageContent'
import { SearchTable, useSearchTableQueryState } from '../../ui/SearchTable'
import { ArchiveIcon, DuplicateIcon, EditIcon, PlusIcon } from '../../components/icons'
import { JourneyForm } from './JourneyForm'
import { Menu, MenuItem, Tag } from '../../ui'
import { ProjectContext } from '../../contexts'
import { useTranslation } from 'react-i18next'
import type { Journey } from '../../types'
import type { UUID } from '@/types/common'

export const JourneyTag = ({ status }: Pick<Journey, 'status'>) => {
    const { t } = useTranslation()
    const variant = status === 'published' ? 'success' : 'plain'
    const title = t(status)
    return <Tag variant={variant}>{title}</Tag>
}

export default function Journeys() {
    const [project] = useContext(ProjectContext)
    const { t } = useTranslation()
    const navigate = useNavigate()
    const [open, setOpen] = useState<null | 'create'>(null)
    const state = useSearchTableQueryState(useCallback(async params => await api.journeys.search(project.id, params), [project.id]))

    const handleEditJourney = async (id: UUID) => {
        await navigate(id.toString())
    }

    const handleDuplicateJourney = async (id: UUID) => {
        const journey = await api.journeys.duplicate(project.id, id)
        await navigate(journey.id.toString())
    }

    const handleArchiveJourney = async (id: UUID) => {
        await api.journeys.delete(project.id, id)
        await state.reload()
    }

    return (
        <PageContent
            title={t('journeys')}
            actions={
                <Dialog open={!!open} onOpenChange={(isOpen) => setOpen(isOpen ? 'create' : null)}>
                    <DialogTrigger asChild>
                        <Button size="lg">
                            <PlusIcon />
                            {t('create_journey')}
                        </Button>
                    </DialogTrigger>
                    <DialogContent>
                        <DialogHeader>
                            <DialogTitle>{t('create_journey')}</DialogTitle>
                        </DialogHeader>
                        <JourneyForm
                            onSaved={async journey => {
                                setOpen(null)
                                await navigate(journey.id.toString())
                            }}
                        />
                    </DialogContent>
                </Dialog>
            }
        >
            <SearchTable
                {...state}
                columns={[
                    {
                        key: 'name',
                        title: t('name'),
                        minWidth: '150px',
                    },
                    {
                        key: 'status',
                        title: t('status'),
                        cell: ({ item }) => <JourneyTag status={item.status} />,
                    },
                    {
                        key: 'created_at',
                        title: t('created_at'),
                    },
                    {
                        key: 'updated_at',
                        title: t('updated_at'),
                    },
                    {
                        key: 'options',
                        title: t('options'),
                        cell: ({ item: { id } }) => (
                            <Menu size="min">
                                <MenuItem onClick={async () => await handleEditJourney(id)}>
                                    <EditIcon />{t('edit')}
                                </MenuItem>
                                <MenuItem onClick={async () => await handleDuplicateJourney(id)}>
                                    <DuplicateIcon />{t('duplicate')}
                                </MenuItem>
                                <MenuItem onClick={async () => await handleArchiveJourney(id)}>
                                    <ArchiveIcon />{t('archive')}
                                </MenuItem>
                            </Menu>
                        ),
                    },
                ]}
                onSelectRow={async r => { await navigate(r.id.toString()) }}
                enableSearch
                tagEntity="journeys"
            />
        </PageContent>
    )
}
