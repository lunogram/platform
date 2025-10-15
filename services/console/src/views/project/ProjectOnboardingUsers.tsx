import { useParams } from 'react-router'
import { useTranslation } from 'react-i18next'
import Button, { LinkButton } from '../../ui/Button'
import api from '../../api'
import { UUID } from 'crypto'
import { useEffect, useState } from 'react'
import TextInput from '../../ui/form/TextInput'

export default function ProjectOnboarding() {
    const { t } = useTranslation()
    const { projectId } = useParams<{ projectId: string }>()
    const [apiKey, setApiKey] = useState<string | undefined>(undefined)

    useEffect(() => {
        const createApiKey = async () => {
            if (projectId) {
                const key = await api.apiKeys.create(projectId as UUID, { name: 'External', scope: 'public' })
                setApiKey(key.value)
            }
        }
        createApiKey().catch(console.error)
    }, [projectId])

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

            <section>
                <h3>{t('onboarding_users_api_key_title')}</h3>
                <div>
                    <TextInput type="text" name="apiKey" label="" value={apiKey} readOnly />
                </div>
            </section>

            <section className="users-sync">
                <p>
                    {t('onboarding_awaiting_users')}
                    <div className="circle pulse"></div>
                </p>
            </section>

            <LinkButton to={`/projects/${projectId}/onboarding/journey`} variant="secondary">{t('skip')}</LinkButton>
        </div>
    )
}
