import { useParams } from 'react-router'
import { useTranslation } from 'react-i18next'
import { LinkButton } from '../../ui/Button'
// import api from '../../api'
// import { UUID } from 'crypto'
import { useState } from 'react'

export default function ProjectOnboarding() {
    // const navigate = useNavigate()
    const { t } = useTranslation()
    const { projectId } = useParams<{ projectId: string }>()
    // const [isLoading, setIsLoading] = useState(false)

    const [tools, setTools] = useState([
        { id: 'wordpress', name: 'WordPress', active: false },
        { id: 'shopify', name: 'Shopify', icon: 'https://cdn3.iconfinder.com/data/icons/social-media-2068/64/_shopping-512.png', active: false },
        { id: 'javascript', name: 'JavaScript', active: false },
        { id: 'mailchimp', name: 'Mailchimp', active: false },
        { id: 'hubspot', name: 'HubSpot', active: false },
        { id: 'python', name: 'Python', active: false },
        { id: 'odoo', name: 'Odoo', active: false },
        { id: 'php', name: 'PHP', active: false },
    ])

    function toggleTool(id: string) {
        setTools(prev =>
            prev.map(tool =>
                tool.id === id ? { ...tool, active: !tool.active } : tool,
            ),
        )
    }

    // async function createOnboardingJourney() {
    //     setIsLoading(true)
    //     try {
    //         // const journey = await api.journeys.create(projectId as UUID, {
    //         //     name: 'Onboarding',
    //         //     description: 'Getting started with your first journey',
    //         //     template_id: 'onboarding',
    //         //     status: 'draft',
    //         // })

    //         await navigate(`/projects/${projectId}/onboarding/users`)
    //     } finally {
    //         setIsLoading(false)
    //     }
    // }

    return (
        <div className="tools-step">
            <h1>{t('onboarding_tools_title')}</h1>
            <p>{t('onboarding_tools_description')}</p>

            <div className="tools-grid">
                {tools.map((tool) => (
                    <button
                        key={tool.id}
                        onClick={() => toggleTool(tool.id)}
                        className={tool.active ? 'active' : ''}
                    >
                        {tool.icon && <img src={tool.icon} alt={tool.name} />}
                        <span>{tool.name}</span>
                    </button>
                ))}
            </div>

            <div className="actions">
                <LinkButton to={`/projects/${projectId}/onboarding/users`}>Next</LinkButton>
                {/* <Button onClick={createOnboardingJourney} isLoading={isLoading} variant="secondary">{t('skip')}</Button> */}
            </div>
        </div>
    )
}
