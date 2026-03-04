import { useCallback, useContext, useState } from 'react'
import { useNavigate, useParams } from 'react-router'
import { useTranslation } from 'react-i18next'
import { ChevronLeft } from 'lucide-react'
import { NIL } from 'uuid'
import api from '../../api'
import { ProjectContext } from '../../contexts'
import { useResolver } from '../../hooks'
import { snakeToTitle } from '../../utils'
import { IntegrationForm } from '../settings/IntegrationModal'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from '@/components/ui/card'
import type { UUID } from '@/types/common'
import type { ProviderMeta } from '../../types'

export default function ProjectOnboardingIntegration() {
    const navigate = useNavigate()
    const { t } = useTranslation()
    const { projectId = NIL as UUID } = useParams<{ projectId: UUID }>()
    const [project, setProject] = useContext(ProjectContext)
    const [meta, setMeta] = useState<ProviderMeta | undefined>()

    const [options] = useResolver(
        useCallback(async () => await api.providers.options(projectId), [projectId]),
    )

    async function handleSkip() {
        await navigate(`/projects/${projectId}/onboarding/users`)
    }

    return (
        <Card className="w-full min-w-[400px] max-w-[600px]">
            <CardHeader>
                <CardTitle className="text-lg">{t('onboarding_integration_title')}</CardTitle>
                <CardDescription>
                    {t('onboarding_integration_description')}
                </CardDescription>
            </CardHeader>
            <CardContent>
                {!meta ? (
                    <div className="grid grid-cols-2 gap-3 sm:grid-cols-3">
                        {options?.map((option) => (
                            <button
                                key={`${option.group}${option.type}`}
                                type="button"
                                className="flex flex-col items-center gap-2 rounded-lg border p-4 text-center transition-colors hover:bg-accent hover:text-accent-foreground"
                                onClick={() => setMeta(option)}
                            >
                                {option.icon && (
                                    <img
                                        src={option.icon}
                                        alt={option.name}
                                        className="h-10 w-10 rounded-md"
                                    />
                                )}
                                <div>
                                    <p className="text-sm font-medium">{option.name}</p>
                                    <p className="text-xs text-muted-foreground">
                                        {snakeToTitle(option.group)}
                                    </p>
                                </div>
                            </button>
                        ))}
                    </div>
                ) : (
                    <>
                        <Button
                            variant="ghost"
                            size="sm"
                            className="mb-4 w-fit"
                            onClick={() => setMeta(undefined)}
                        >
                            <ChevronLeft className="mr-1 h-4 w-4" />
                            {t('integrations')}
                        </Button>
                        <IntegrationForm
                            project={project}
                            meta={meta}
                            onChange={async () => {
                                const updatedProject = await api.projects.get(projectId)
                                setProject(updatedProject)
                                await navigate(`/projects/${projectId}/onboarding/users`)
                            }}
                        />
                    </>
                )}
            </CardContent>
            {!meta && (
                <CardFooter>
                    <Button variant="outline" onClick={handleSkip}>
                        {t('skip')}
                    </Button>
                </CardFooter>
            )}
        </Card>
    )
}
