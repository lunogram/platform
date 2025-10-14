import App from '../../app'
import { SendRequest } from '../../../oapi/webhooks'

export async function sendMessage(
    externalId: string,
    message: SendRequest,
): Promise<void> {
    if (!App.main.env.webhooks.send) {
        throw new Error('No send webhook configured')
    }

    const controller = new AbortController()
    const timeoutId = setTimeout(() => controller.abort(), App.main.env.webhooks.send.timeoutMs)

    try {
        const url = new URL(App.main.env.webhooks.send.url)
        url.searchParams.set('external_id', externalId)

        const res = await fetch(url, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify(message),
            signal: controller.signal,
        })

        clearTimeout(timeoutId)
        if (!res.ok) throw new Error(`Webhook error: ${res.statusText}`)
    } catch (error) {
        clearTimeout(timeoutId)
        throw error
    }
}
