import { useNavigate } from 'react-router'
import { useTranslation } from 'react-i18next'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import ProjectForm from '../project/ProjectForm'

export default function OnboardingProject() {
    const { t } = useTranslation()
    const navigate = useNavigate()
    return (
        <Card className="w-full min-w-[400px] max-w-[600px]">
            <CardHeader>
                <CardTitle className="text-2xl">{t('onboarding_project_setup_title')}</CardTitle>
                <CardDescription>
                    {t('onboarding_project_setup_description')}
                </CardDescription>
            </CardHeader>
            <CardContent>
                <ProjectForm onSave={async ({ id }) => { await navigate('/projects/' + id) }} />
            </CardContent>
        </Card>
    )
}
