import { useCallback, useContext, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { ProjectContext, UserContext } from '../../contexts'
import { SearchTable, useSearchTableState } from '../../ui/SearchTable'
import { Modal, JsonPreview } from '../../ui'
import { useRoute } from '../router'
import oapiClient, { type Organization } from '../../oapi/client'
import type { SearchParams, SearchResult } from '../../types'

export default function UserDetailOrganizations() {
    const { t } = useTranslation()
    const route = useRoute()
    const [project] = useContext(ProjectContext)
    const [user] = useContext(UserContext)

    const fetchOrganizations = useCallback(async (params: SearchParams): Promise<SearchResult<Organization> | null> => {
        const { data } = await oapiClient.GET('/api/admin/projects/{projectID}/subjects/users/{userID}/subject-organizations', {
            params: {
                path: {
                    projectID: project.id,
                    userID: user.id,
                },
                query: {
                    limit: params.limit,
                    offset: params.offset,
                    q: params.q || undefined,
                },
            },
        })
        if (!data) return null
        return {
            results: data.results,
            nextCursor: data.next_cursor ?? '',
            limit: params.limit,
        }
    }, [project.id, user.id])

    const state = useSearchTableState<Organization>(fetchOrganizations)

    const [selectedOrganization, setSelectedOrganization] = useState<Organization | null>(null)

    const getOrganizationDisplayName = (org: Organization) => {
        if (org.name) return org.name
        return org.external_id ?? org.id
    }

    return (
        <>
            <SearchTable
                {...state}
                title={t('organizations')}
                columns={[
                    {
                        key: 'name',
                        title: t('name'),
                        render: (org) => getOrganizationDisplayName(org),
                    },
                    { key: 'external_id', title: t('external_id') },
                    {
                        key: 'created_at',
                        title: t('created_at'),
                    },
                ]}
                onSelectRow={(org) => setSelectedOrganization(org)}
            />

            <Modal
                open={selectedOrganization !== null}
                onClose={() => setSelectedOrganization(null)}
                title={selectedOrganization ? getOrganizationDisplayName(selectedOrganization) : ''}
                size="large"
            >
                {selectedOrganization && (
                    <div className="space-y-4">
                        <div>
                            <h4 className="font-medium mb-2">{t('organization_data')}</h4>
                            <JsonPreview value={selectedOrganization.data} />
                        </div>
                        <div className="pt-4">
                            <button
                                className="text-sm text-blue-600 hover:text-blue-800 underline"
                                onClick={() => {
                                    setSelectedOrganization(null)
                                    route(`organizations/${selectedOrganization.id}`)
                                }}
                            >
                                {t('view_organization')}
                            </button>
                        </div>
                    </div>
                )}
            </Modal>
        </>
    )
}
