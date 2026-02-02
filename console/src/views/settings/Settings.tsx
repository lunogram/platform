import { Outlet } from 'react-router'
import PageContent from '../../ui/PageContent'
import { NavigationTabs } from '../../ui/Tabs'
import { ProjectRoleRequired } from '../project/ProjectRoleRequired'
import { useTranslation } from 'react-i18next'

export default function Settings() {
    const { t } = useTranslation()
    const tabs = [
        {
            key: 'general',
            to: '',
            end: true,
            children: t('general'),
        },
        {
            key: 'locales',
            to: 'locales',
            children: t('locales'),
        },
        {
            key: 'api-keys',
            to: 'api-keys',
            children: t('api_keys'),
        },
        {
            key: 'integrations',
            to: 'integrations',
            children: t('integrations'),
        },
        {
            key: 'subscriptions',
            to: 'subscriptions',
            children: t('subscriptions'),
        },
    ]

    return (
        <ProjectRoleRequired minRole="admin">
            <PageContent title={t('settings')}>
                <NavigationTabs tabs={tabs} />
                <Outlet />
            </PageContent>
        </ProjectRoleRequired>
    )
}
