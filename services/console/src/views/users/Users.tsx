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
import TextInput from '../../ui/form/TextInput'
import { SingleSelect } from '../../ui/form/SingleSelect'
import { PlusIcon, TrashIcon } from '../../components/icons'
import type { UUID } from '@/types/common'
import { NIL } from 'uuid'
import type { User } from '../../types'

import './Users.css'

// eslint-disable-next-line @typescript-eslint/no-namespace
export declare namespace Intl {
    type Key = 'calendar' | 'collation' | 'currency' | 'numberingSystem' | 'timeZone' | 'unit'
    function supportedValuesOf(input: Key): string[]

    interface DateTimeFormat {

        format(date?: Date | number): string

        resolvedOptions(): ResolvedDateTimeFormatOptions
    }

    interface ResolvedDateTimeFormatOptions {
        locale: string
        timeZone: string
        timeZoneName?: string
    }

    // eslint-disable-next-line no-var
    var DateTimeFormat: {
        new(locales?: string | string[]): DateTimeFormat
        (locales?: string | string[]): DateTimeFormat
    }
}

export default function UserTabs() {
    const { projectId = NIL as UUID } = useParams<{ projectId: UUID }>()
    const { t } = useTranslation()
    const route = useRoute()
    const timeZones = Intl.supportedValuesOf('timeZone')
    const locale = navigator.languages[0]?.split('-')[0] ?? 'en'

    const state = useSearchTableQueryState(useCallback(async params => await api.users.search(projectId, params), [projectId]))
    const [isBulkRemovalOpen, setIsBulkRemovalOpen] = useState(false)
    const [isCreateUserOpen, setIsCreateUserOpen] = useState(false)

    const defaultUser = {
        timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
        locale,
    }

    const createUser = async (user: User) => {
        const { full_name, ...rest } = user
        const newUser: User = {
            ...rest,
            anonymous_id: crypto.randomUUID() as UUID,
            ...(full_name
                ? { data: { full_name } }
                : { data: user.data }),
        }

        await api.users.create(projectId, newUser)
        await state.reload()

        setIsCreateUserOpen(false)
    }

    const bulkRemoveUsers = async (file: FileList) => {
        await api.users.deleteImport(projectId, file[0])
        await state.reload()
        setIsBulkRemovalOpen(false)
    }

    return <PageContent
        title={t('users')}
        actions={
            <>
                <Button icon={<TrashIcon />}
                    onClick={() => setIsBulkRemovalOpen(true)}
                    variant="destructive">{t('delete_users')}
                </Button>
                <Button icon={<PlusIcon />}
                    onClick={() => setIsCreateUserOpen(true)}>{t('create_user')}
                </Button>
            </>
        }>
        <SearchTable
            {...state}
            columns={[
                { key: 'full_name', title: t('name') },
                { key: 'external_id', title: t('external_id') },
                { key: 'email', title: t('email') },
                { key: 'phone', title: t('phone') },
                { key: 'locale', title: t('locale.singular') },
                { key: 'created_at', title: t('created_at'), sortable: true },
            ]}
            onSelectRow={({ id }) => route(`users/${id}`)}
            enableSearch
            searchPlaceholder={t('search_users')}
        />

        <Modal
            open={isCreateUserOpen}
            onClose={() => setIsCreateUserOpen(false)}
            title={t('create_user')}>
            <FormWrapper<User>
                defaultValues={defaultUser}
                onSubmit={async (form) => await createUser(form)}
                submitLabel={t('create')}
            >
                {form => <>
                    <TextInput.Field form={form} name="full_name" label={t('full_name')} />
                    <TextInput.Field form={form} name="email" label={t('email')} />
                    <TextInput.Field form={form} name="phone" label={t('phone')} />
                    <SingleSelect.Field
                        form={form}
                        options={timeZones}
                        name="timezone"
                        label={t('timezone')}
                    />
                    <TextInput.Field form={form} name="locale" label={t('locale.singular')} />
                </>}
            </FormWrapper>
        </Modal>

        <Modal
            open={isBulkRemovalOpen}
            onClose={() => setIsBulkRemovalOpen(false)}
            title={t('delete_users')}>
            <FormWrapper<{ file: FileList }>
                onSubmit={async (form) => await bulkRemoveUsers(form.file)}
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
