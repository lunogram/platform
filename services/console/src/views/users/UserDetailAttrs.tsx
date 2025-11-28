import { useContext, useState, useEffect } from 'react'
import { ProjectContext, UserContext } from '../../contexts'
import Heading from '../../ui/Heading'
import JsonPreview from '../../ui/JsonPreview'
import { useTranslation } from 'react-i18next'
import { getEditableUserFields } from '../../ui/utils'
import api from '../../api'
import { Button } from '@/components/ui/button'
import type { User } from '../../types'

export default function UserDetail() {
    const { t } = useTranslation()
    const [project] = useContext(ProjectContext)
    const [user] = useContext(UserContext)
    const [editableUser, setEditableUser] = useState(() => getEditableUserFields(user))
    const [saving, setSaving] = useState(false)
    const [dirty, setDirty] = useState(false)

    // keep local state in sync when user changes externally
    useEffect(() => {
        setEditableUser(getEditableUserFields(user))
        setDirty(false)
    }, [user])

    function handleChange(path: Array<string | number>, _: unknown, value: unknown) {
        // Update nested field in editableUser
        setEditableUser(prev => {
            const clone = structuredClone(prev)
            let target: Record<string, unknown> = clone
            for (let i = 0; i < path.length - 1; i++) {
                target = target[path[i]] as Record<string, unknown>
            }
            target[path[path.length - 1]] = value
            return clone
        })
        setDirty(true)
    }

    async function handleSave() {
        try {
            setSaving(true)
            await api.users.update(project.id, user.id, editableUser as User)
            setDirty(false)
        } finally {
            setSaving(false)
        }
    }

    return (
        <div className="user-details-attrs">
            <Heading size="h3" title={t('details')} />

            <section className="legacy-container">
                <JsonPreview
                    editable
                    onChange={handleChange}
                    value={editableUser}
                />
            </section>

            <Button
                disabled={!dirty || saving}
                isLoading={saving}
                onClick={handleSave}
            >
                {t('save')}
            </Button>
        </div>
    )
}
