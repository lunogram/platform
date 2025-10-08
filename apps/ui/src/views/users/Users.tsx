import { useCallback, useState } from 'react'
import { useParams } from 'react-router'
import api from '../../api'
import PageContent from '../../ui/PageContent'
import { SearchTable, useSearchTableQueryState } from '../../ui/SearchTable'
import { useRoute } from '../router'
import { useTranslation } from 'react-i18next'
import { Button, Modal } from '../../ui'
import FormWrapper from '../../ui/form/FormWrapper'
import UploadField from '../../ui/form/UploadField'
import { TrashIcon } from '../../ui/icons'

export default function UserTabs() {
    const { projectId = '' } = useParams()
    const { t } = useTranslation()
    const route = useRoute()
    const state = useSearchTableQueryState(useCallback(async params => await api.users.search(projectId, params), [projectId]))
    const [isUploadOpen, setIsUploadOpen] = useState(false)

    const removeUsers = async (file: FileList) => {
        await api.users.deleteImport(projectId, file[0])
        await state.reload()
        setIsUploadOpen(false)
    }

    return <PageContent
        title={t('users')}
        actions={
            <Button icon={<TrashIcon />}
                onClick={() => setIsUploadOpen(true)}
                variant="destructive">{t('delete_users')}</Button>
        }>
        <SearchTable
            {...state}
            columns={[
                { key: 'full_name', title: t('name') },
                { key: 'external_id', title: t('external_id') },
                { key: 'email', title: t('email') },
                { key: 'phone', title: t('phone') },
                { key: 'locale', title: t('locale') },
                { key: 'created_at', title: t('created_at'), sortable: true },
            ]}
            onSelectRow={({ id }) => route(`users/${id}`)}
            enableSearch
            searchPlaceholder={t('search_users')}
        />

        <Modal
            open={isUploadOpen}
            onClose={() => setIsUploadOpen(false)}
            title={t('delete_users')}>
            <FormWrapper<{ file: FileList }>
                onSubmit={async (form) => await removeUsers(form.file)}
                submitLabel={t('delete')}
            >
                {form => <>
                    <p>{t('delete_users_instructions')}</p>
                    <UploadField form={form} name="file" label={t('file')} required />
                </>}
            </FormWrapper>
        </Modal>
    </PageContent>
}
