import { useNavigate, useParams } from 'react-router'
import { Trans, useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import api from '../../api'
import type { UUID } from '@/types/common'
import { useState } from 'react'
import FormWrapper from '../../ui/form/FormWrapper'
import UploadField from '../../ui/form/UploadField'
import { NIL } from 'uuid'

export default function ProjectOnboarding() {
    const navigate = useNavigate()
    const { t } = useTranslation()
    const { projectId = NIL as UUID } = useParams<{ projectId: UUID }>()
    const [skipLoading, setSkipLoading] = useState(false)
    const [nextLoading, setNextLoading] = useState(false)

    const next = async (file: FileList) => {
        setNextLoading(true)
        try {
            if (file) {
                await api.users.addImport(projectId, file[0])
            } else {
                await createInitialUser()
            }
            await navigate(`/projects/${projectId}/onboarding/getting-started`)
        } finally {
            setNextLoading(false)
        }
    }

    async function skip() {
        setSkipLoading(true)
        try {
            await createInitialUser()
            await navigate(`/projects/${projectId}/onboarding/getting-started`)
        } finally {
            setSkipLoading(false)
        }
    }

    async function createInitialUser() {
        const admin = await api.admins.whoami()
        if (!admin) return

        await api.users.create(projectId, {
            anonymous_id: crypto.randomUUID(),
            data: {
                first_name: admin.first_name,
                last_name: admin.last_name,
            },
            email: admin.email,
        })
    }

    return (
        <div>
            <section>
                <h1>{t('onboarding_users_title')}</h1>
                <p>{t('onboarding_users_description')}</p>
            </section>

            <hr />

            <FormWrapper<{ file: FileList }>
                onSubmit={async (form) => await next(form.file)}
                showSubmitButton={false}
            >
                {form => <>
                    <p>
                        <Trans
                            i18nKey="onboarding_project_users_template"
                            components={{
                                download: <a href="/templates/users.csv" download="users.csv" className="underline" />,
                            }}
                        />
                    </p>

                    <UploadField form={form} name="file" label={t('users')} required />

                    <div className="flex gap-2 mt-4">
                        <Button isLoading={nextLoading} type="submit">{t('next')}</Button>
                        <Button onClick={skip} isLoading={skipLoading} variant="secondary">{t('skip')}</Button>
                    </div>
                </>}
            </FormWrapper>

        </div>
    )
}
