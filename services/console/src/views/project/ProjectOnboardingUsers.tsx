import { useNavigate, useParams } from 'react-router'
import { useTranslation } from 'react-i18next'
import Button from '../../ui/Button'
import api from '../../api'
import { UUID } from 'crypto'
import { useEffect, useState } from 'react'
import TextInput from '../../ui/form/TextInput'

export default function ProjectOnboarding() {
    const navigate = useNavigate()
    const { t } = useTranslation()
    const { projectId } = useParams<{ projectId: UUID }>()
    const [apiKey, setApiKey] = useState<string | undefined>(undefined)
    const [loading, setLoading] = useState(false)

    useEffect(() => {
        const createApiKey = async () => {
            if (projectId) {
                const key = await api.apiKeys.create(projectId, { name: 'External', scope: 'public' })
                setApiKey(key.value)
            }
        }
        createApiKey().catch(console.error)
    }, [projectId])

    async function skip() {
        setLoading(true)
        try {
            if (!projectId) return

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

            await navigate(`/projects/${projectId}/onboarding/journey`)
        } finally {
            setLoading(false)
        }
    }

    return (
        <div className="users-step">
            <section>
                <h1>{t('onboarding_users_title')}</h1>
                <p>{t('onboarding_users_description')}</p>
                <div className="actions">
                    <Button disabled variant="secondary" size="small">Plugins</Button>
                    <Button disabled variant="secondary" size="small">SDK</Button>
                    <Button disabled variant="secondary" size="small">CSV</Button>
                </div>
            </section>

            {apiKey && (
                <section>
                    <h3>{t('onboarding_users_api_key_title')}</h3>
                    <div>
                        <TextInput type="text" name="apiKey" label="" value={apiKey} readOnly />
                    </div>
                </section>
            )}

            <section className="users-sync">
                <p>
                    {t('onboarding_awaiting_users')}
                    <div className="circle pulse"></div>
                </p>
            </section>

            <Button onClick={skip} isLoading={loading} variant="secondary">{t('skip')}</Button>
        </div>
    )
}
