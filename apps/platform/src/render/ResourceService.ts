import Resource, { ResourceParams, ResourceType } from './Resource'

export const allResources = async (projectId: UUID, type?: ResourceType): Promise<Resource[]> => {
    return await Resource.all(qb => {
        if (type) {
            qb.where('type', type)
        }
        return qb.where('project_id', projectId)
    })
}

export const getResource = async (id: UUID, projectId: UUID) => {
    return await Resource.find(id, qb => qb.where('project_id', projectId))
}

export const createResource = async (projectId: UUID, params: ResourceParams) => {
    return await Resource.insertAndFetch({
        ...params,
        project_id: projectId,
    })
}

export const deleteResource = async (id: UUID, projectId: UUID) => {
    return await Resource.deleteById(id, qb => qb.where('project_id', projectId))
}
