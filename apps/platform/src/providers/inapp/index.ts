import { loadPushChannel } from '../push'
import InAppChannel from './InAppChannel'

export const loadInAppChannel = async (providerId: UUID, projectId: UUID) => {
    const channel = await loadPushChannel(providerId, projectId)
    if (!channel) return
    return new InAppChannel(channel)
}
