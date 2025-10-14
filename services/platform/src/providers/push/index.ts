import { UUID } from 'crypto'
import { loadProvider } from '../ProviderRepository'
import LocalPushProvider from './LocalPushProvider'
import WebhookPushProvider from './WebhookPushProvider'
import LoggerPushProvider from './LoggerPushProvider'
import PushChannel from './PushChannel'
import { PushProvider, PushProviderName } from './PushProvider'

type PushProviderDerived = { new(): PushProvider } & typeof PushProvider
export const typeMap: Record<string, PushProviderDerived> = {
    local: LocalPushProvider,
    webhook: WebhookPushProvider,
    logger: LoggerPushProvider,
}

export const providerMap = (record: { type: PushProviderName }): PushProvider => {
    if (!typeMap[record.type]) throw new Error(`Unknown push provider type: ${record.type}`)
    return typeMap[record.type].fromJson(record)
}

export const loadPushChannel = async (providerId: UUID, projectId: UUID): Promise<PushChannel | undefined> => {
    const provider = await loadProvider(providerId, providerMap, projectId)
    if (!provider) return
    return new PushChannel(provider)
}

export const pushProviders = Object.values(typeMap)
