import { useNavigate, useParams } from 'react-router'
import { useTranslation } from 'react-i18next'
import Button from '../../ui/Button'
import api from '../../api'
import { UUID } from 'crypto'
import { useContext, useEffect, useState } from 'react'
import { NIL } from 'uuid'
import { ProjectContext } from '../../contexts'

export default function ProjectOnboarding() {
    const navigate = useNavigate()
    const { t } = useTranslation()
    const { projectId = NIL as UUID } = useParams<{ projectId: UUID }>()
    const [project, setProject] = useContext(ProjectContext)
    const [isLoading, setIsLoading] = useState(false)

    const [tools, setTools] = useState([
        { id: 'wordpress', name: 'WordPress', icon: 'https://lunogram.com/sources/wordpress.svg', active: false },
        { id: 'shopify', name: 'Shopify', icon: 'https://lunogram.com/sources/shopify.svg', active: false },
        { id: 'javascript', name: 'JavaScript', icon: 'https://lunogram.com/sources/javascript.svg', active: false },
        { id: 'mailchimp', name: 'Mailchimp', icon: 'https://lunogram.com/sources/mailchimp.svg', active: false },
        { id: 'hubspot', name: 'HubSpot', icon: 'https://lunogram.com/sources/hubspot.svg', active: false },
        { id: 'python', name: 'Python', icon: 'https://lunogram.com/sources/python.svg', active: false },
        { id: 'odoo', name: 'Odoo', icon: 'https://lunogram.com/sources/odoo.svg', active: false },
        { id: 'php', name: 'PHP', icon: 'https://lunogram.com/sources/php.svg', active: false },
    ])

    useEffect(() => {
        if (!project) return
        setTools(tools.map(tool => ({
            ...tool,
            active: project.tools ? project.tools.includes(tool.id) : false,
        })))
    }, [project])

    function toggleTool(id: string) {
        setTools(prev =>
            prev.map(tool =>
                tool.id === id ? { ...tool, active: !tool.active } : tool,
            ),
        )
    }

    async function saveTools() {
        setIsLoading(true)
        try {
            const { name, description, locale, timezone, text_opt_out_message, text_help_message, link_wrap_email, link_wrap_push } = project
            const params = { name, description, locale, timezone, text_opt_out_message, text_help_message, link_wrap_email, link_wrap_push }

            const updatedProject = await api.projects.update(projectId, {
                ...params,
                tools: tools.filter(tool => tool.active).map(tool => tool.id),
            })

            setProject(updatedProject)
            await navigate(`/projects/${projectId}/onboarding/users`)
        } finally {
            setIsLoading(false)
        }
    }

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
                <Button onClick={saveTools} isLoading={isLoading}>{t('next')}</Button>
            </div>
        </div>
    )
}
