# Web Push Provider

Send push notifications to web browsers using the Web Push Protocol.

## Overview

This provider enables sending push notifications to:
- Chrome (Desktop & Mobile)
- Firefox (Desktop & Mobile)
- Safari (Desktop & Mobile)
- Edge (Desktop & Mobile)
- Opera (Desktop & Mobile)

## Prerequisites

1. **VAPID Keys**: You need a VAPID key pair for authentication. Your platform already has these in the database!

### Getting Your VAPID Keys

Your VAPID keys are stored in the `vapid_keys` table. To get them:

```sql
SELECT public_key, private_key 
FROM vapid_keys 
WHERE name = 'default' 
AND deleted_at IS NULL;
```

Or use the platform API (if available) to retrieve them.

If you don't have VAPID keys yet, the platform automatically creates a default pair on startup via `CreateVapidKeysIfNotExist()`.

## Configuration

When setting up this provider in your UI, you'll need to configure:

```json
{
  "data": {
    "vapidPublicKey": "YOUR_VAPID_PUBLIC_KEY_HERE",
    "vapidPrivateKey": "YOUR_VAPID_PRIVATE_KEY_HERE",
    "vapidEmail": "mailto:admin@example.com"
  }
}
```

### Configuration Fields

| Field | Required | Description |
|-------|----------|-------------|
| `vapidPublicKey` | ✅ Yes | Your VAPID public key (base64url encoded) |
| `vapidPrivateKey` | ✅ Yes | Your VAPID private key (base64url encoded) |
| `vapidEmail` | ✅ Yes | Contact email for VAPID (must start with `mailto:`) |

## How It Works

1. **User subscribes** to push notifications in their browser
2. **Browser generates** a push subscription with endpoint and encryption keys
3. **Your app registers** the subscription via the platform API
4. **Platform stores** the subscription in the `devices` table as `device_credentials`
5. **When triggered**, the platform:
   - Calls `ComposePush()` to collect Web Push subscriptions
   - Sends the payload to this provider
   - Provider sends to each subscription endpoint

## Testing

### 1. Register a Test Subscription

Create a simple HTML page to test:

```html
<!DOCTYPE html>
<html>
<head>
    <title>Web Push Test</title>
</head>
<body>
    <h1>Web Push Test</h1>
    <button onclick="subscribe()">Subscribe to Push</button>
    <pre id="subscription"></pre>

    <script>
    const vapidPublicKey = 'YOUR_VAPID_PUBLIC_KEY';

    async function subscribe() {
        // Request permission
        const permission = await Notification.requestPermission();
        if (permission !== 'granted') {
            alert('Permission denied');
            return;
        }

        // Register service worker
        const registration = await navigator.serviceWorker.register('/sw.js');
        
        // Subscribe to push
        const subscription = await registration.pushManager.subscribe({
            userVisibleOnly: true,
            applicationServerKey: urlBase64ToUint8Array(vapidPublicKey)
        });

        // Display subscription
        const subJson = subscription.toJSON();
        document.getElementById('subscription').textContent = 
            JSON.stringify(subJson, null, 2);

        // Send to your API
        await fetch('https://your-api.com/api/v1/users/me/devices', {
            method: 'POST',
            headers: { 
                'Content-Type': 'application/json',
                'Authorization': 'Bearer YOUR_TOKEN'
            },
            body: JSON.stringify({
                deviceId: 'browser-' + Date.now(),
                deviceCredentials: subJson
            })
        });

        alert('Subscribed!');
    }

    function urlBase64ToUint8Array(base64String) {
        const padding = '='.repeat((4 - base64String.length % 4) % 4);
        const base64 = (base64String + padding)
            .replace(/\-/g, '+')
            .replace(/_/g, '/');
        const rawData = window.atob(base64);
        const outputArray = new Uint8Array(rawData.length);
        for (let i = 0; i < rawData.length; ++i) {
            outputArray[i] = rawData.charCodeAt(i);
        }
        return outputArray;
    }
    </script>
</body>
</html>
```

Create `sw.js` (Service Worker):

```javascript
self.addEventListener('push', function(event) {
    const data = event.data.json();
    
    const options = {
        body: data.body,
        icon: data.image || '/icon.png',
        badge: data.badge,
        data: data.data
    };

    event.waitUntil(
        self.registration.showNotification(data.title, options)
    );
});

self.addEventListener('notificationclick', function(event) {
    event.notification.close();
    
    if (event.notification.data && event.notification.data.url) {
        event.waitUntil(
            clients.openWindow(event.notification.data.url)
        );
    }
});
```

### 2. Send a Test Push

Once you have a subscription registered, create a push template with:

```json
{
  "title": "Hello from Web Push!",
  "body": "This is a test notification",
  "data": {
    "url": "https://example.com",
    "action": "test"
  }
}
```

Then trigger a campaign targeting the test user.

### 3. Check Logs

The provider logs detailed information:
- Number of subscriptions being sent to
- Success/failure count for each subscription
- Specific error messages for failures

## Response Codes

The provider handles various HTTP response codes from push services:

| Code | Meaning | Action |
|------|---------|--------|
| 201 | Success | Notification delivered |
| 400 | Bad Request | Check payload format |
| 401 | Unauthorized | Check VAPID keys |
| 404 | Not Found | Subscription doesn't exist |
| 410 | Gone | Subscription expired - remove from DB |
| 413 | Payload Too Large | Reduce payload size |
| 429 | Rate Limited | Retry with backoff |
| 5xx | Server Error | Push service issue - retry |

## Building

To rebuild after making changes:

```bash
cd modules/providers/webpush
make wasm
```

This creates `internal/providers/modules/webpush.wasm` which is automatically loaded by the platform.

## Payload Structure

The provider receives a `PushPayload` from `ComposePush()`:

```go
type PushPayload struct {
    WebPushTargets []WebPushTarget `json:"web_push_targets"`
    Title          string          `json:"title"`
    Body           string          `json:"body"`
    ImageURL       *string         `json:"image_url,omitempty"`
    Data           map[string]any  `json:"data,omitempty"`
    Sound          *string         `json:"sound,omitempty"`
    Badge          *int            `json:"badge,omitempty"`
}
```

The notification sent to browsers looks like:

```json
{
  "title": "Your title",
  "body": "Your message",
  "image": "https://example.com/image.png",
  "badge": 1,
  "sound": "default",
  "data": {
    "custom": "data"
  }
}
```

## Limitations

- **FCM tokens not supported**: This provider only handles Web Push subscriptions. For FCM tokens (mobile apps), you need a separate FCM provider.
- **TTL**: Messages expire after 24 hours if the browser is offline
- **Payload size**: Limited to ~4KB (varies by push service)
- **Rate limits**: Vary by push service (Chrome uses FCM, Firefox uses Mozilla push service)

## Troubleshooting

### "No Web Push subscriptions provided"

This means the user has no Web Push subscriptions registered. Check:
1. User has registered a device via the API
2. Device has `device_credentials` populated (not just `token`)
3. `ComposePush()` is correctly extracting Web Push targets

### "Unauthorized - check VAPID keys"

This means the VAPID keys are invalid or don't match. Verify:
1. Public and private keys match
2. Keys are base64url encoded
3. Keys are from the same VAPID key pair

### "Subscription expired (status 410)"

The browser subscription has expired or been unsubscribed. The device should be removed from the database.

### No notification appears in browser

Check:
1. Browser has notification permission granted
2. Service worker is registered and active
3. Service worker has a `push` event listener
4. Browser DevTools → Application → Service Workers shows it's running

## Learn More

- [Web Push Protocol (RFC 8030)](https://datatracker.ietf.org/doc/html/rfc8030)
- [VAPID (RFC 8292)](https://datatracker.ietf.org/doc/html/rfc8292)
- [Push API (MDN)](https://developer.mozilla.org/en-US/docs/Web/API/Push_API)
- [Service Worker API (MDN)](https://developer.mozilla.org/en-US/docs/Web/API/Service_Worker_API)

## License

MIT
