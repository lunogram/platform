import type { ModalStateProps } from '../../../ui/Modal';
import Modal from '../../../ui/Modal'
import { useContext, useState } from 'react'
import { ProjectContext } from '../../../contexts'
import api from '../../../api'
import './ImageGalleryModal.css'
import type { Resource } from '../../../types'
import { Button, DataTable, Heading } from '../../../ui'
import { useTranslation } from 'react-i18next'
import ResourceFontModal from './ResourceFontModal'
import type { UUID } from '@/types/common'

interface ResourceModalProps extends ModalStateProps {
    resources: Resource[]
    setResources: (resources: Resource[]) => void
}

export default function ResourceModal({ open, onClose, resources, setResources }: ResourceModalProps) {
    const { t } = useTranslation()
    const [project] = useContext(ProjectContext)
    const [showFontCreate, setShowFontCreate] = useState(false)

    const handleRemove = async (id: UUID) => {
        await api.resources.delete(project.id, id)
        setResources(resources.filter(resource => resource.id !== id))
    }

    const handleAddResource = (resource: Resource) => {
        setShowFontCreate(false)
        setResources([...resources, resource])
    }

    return (
        <Modal
            title="Config"
            open={open}
            onClose={onClose}
            size="large">
            <Heading size="h4" title={t('fonts')} actions={
                <Button size="small" onClick={() => setShowFontCreate(true)}>{t('add_font')}</Button>
            } />
            <div className="resources">
                <DataTable
                    items={resources}
                    itemKey={({ item }) => item.id}
                    columns={[
                        {
                            key: 'name',
                            title: t('name'),
                        },
                        {
                            key: 'family',
                            title: 'Font Family',
                            cell: ({ item }) => item.value.value,
                        },
                        {
                            key: 'url',
                            title: 'URL',
                            cell: ({ item }) => item.value.url,
                        },
                        {
                            key: 'options',
                            title: t('options'),
                            cell: ({ item }) => (
                                <Button
                                    size="small"
                                    variant="destructive"
                                    onClick={async () => await handleRemove(item.id)}>
                                    {t('delete')}
                                </Button>
                            ),
                        },
                    ]} />
            </div>

            <ResourceFontModal
                open={showFontCreate}
                onClose={() => setShowFontCreate(false)}
                onInsert={(resource) => handleAddResource(resource)}
            />
        </Modal>
    )
}
