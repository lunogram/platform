import { useNavigate, useParams } from 'react-router'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { UserImportForm } from '@/components/ui/user-import-dialog'
import api from '../../api'
import type { UUID } from '@/types/common'
import { useState } from 'react'
import { NIL } from 'uuid'

export default function ProjectOnboarding() {
    const navigate = useNavigate()
    const { t } = useTranslation()
    const { projectId = NIL as UUID } = useParams<{ projectId: UUID }>()
    const [file, setFile] = useState<File | null>(null)
    const [nextLoading, setNextLoading] = useState(false)
    const [skipLoading, setSkipLoading] = useState(false)

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

    const next = async () => {
        setNextLoading(true)
        try {
            if (file) {
                await api.users.addImport(projectId, file)
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

    return (
        <div>
            <section>
                <h1>{t('onboarding_users_title')}</h1>
                <p>{t('onboarding_users_description')}</p>
            </section>

            <hr />

            <div className="my-4">
                <UserImportForm file={file} onFileChange={setFile} />
            </div>

            <div className="flex gap-2 mt-4">
                <Button onClick={next} isLoading={nextLoading}>{t('next')}</Button>
                <Button onClick={skip} isLoading={skipLoading} variant="secondary">{t('skip')}</Button>
            </div>
        </div>
    )
}
