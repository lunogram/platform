import type { paths } from './webhooks.generated'

export type BootstrapResponse = paths['/webhooks/providers/bootstrap']['post']['responses']['200']['content']['application/json']
export type SendRequest = paths['/webhooks/provider/send']['post']['requestBody']['content']['application/json']