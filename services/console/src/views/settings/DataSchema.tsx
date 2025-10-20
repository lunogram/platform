import { useCallback, useContext, useState } from 'react'
import api from '../../api'
import { ProjectContext } from '../../contexts'
import { SearchTable, useSearchTableState } from '../../ui/SearchTable'
import { snakeToTitle } from '../../utils'
import { useTranslation } from 'react-i18next'
import { Button, Modal } from '../../ui'
import type { RulePath } from '../../types'
import FormWrapper from '../../ui/form/FormWrapper'
import RadioInput from '../../ui/form/RadioInput'
import { toast } from 'react-hot-toast/headless'

export default function DataSchema() {
    const { t } = useTranslation()
    const [project] = useContext(ProjectContext)
    const state = useSearchTableState(useCallback(async params => await api.data.userPaths.search(project.id, params), [project]))
    const [editing, setEditing] = useState<null | Partial<RulePath>>(null)

    const handleRebuildAttributeSchema = async () => {
        await api.data.rebuild(project.id)
        toast.success(t('rebuild_path_suggestions_success'))
        await state.reload()
    }

    return (
        <>
            <SearchTable
                {...state}
                columns={[
                    { key: 'path', title: t('path') },
                    {
                        key: 'type',
                        title: t('type'),
                        cell: ({ item }) => snakeToTitle(item.type),
                    },
                    {
                        key: 'data_type',
                        title: t('data_type'),
                        cell: ({ item }) => snakeToTitle(item.data_type),
                    },
                    {
                        key: 'visibility',
                        title: t('visibility'),
                        cell: ({ item }) => snakeToTitle(item.visibility),
                    },
                ]}
                itemKey={({ item }) => item.id}
                onSelectRow={(row) => setEditing(row)}
                title={t('data_schema')}
                description={t('data_schema_description')}
                actions={
                    <Button onClick={handleRebuildAttributeSchema}>Sync</Button>
                }
            />
            <Modal
                title={editing?.path}
                open={Boolean(editing)}
                onClose={() => setEditing(null)}
            >
                {
                    editing && (
                        <FormWrapper<RulePath>
                            onSubmit={
                                async ({ id, visibility }) => {
                                    await api.data.userPaths.update(project.id, id, { visibility })
                                    await state.reload()
                                    setEditing(null)
                                }
                            }
                            defaultValues={editing}
                        >
                            {form => <>
                                <RadioInput.Field
                                    form={form}
                                    name="visibility"
                                    label={t('visibility')}
                                    options={[
                                        { key: 'public', label: 'Public' },
                                        { key: 'hidden', label: 'Hidden' },
                                        { key: 'classified', label: 'Classified' },
                                    ]}
                                />
                            </>}
                        </FormWrapper>
                    )
                }
            </Modal>
        </>
    )
}
