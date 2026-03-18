self.addEventListener("install", (event) => {
    self.skipWaiting() // don't wait for old SW to die
    console.log("SW installed and ready to take over")
})

self.addEventListener("activate", (event) => {
    event.waitUntil(clients.claim())
    console.log("SW activated and claimed clients")
})

console.log("SW FILE LOADED - if you dont see this the file is fucked")
self.addEventListener("push", (event) => {
    let data = {}

    // Try to parse as JSON, fallback to plain text
    if (event.data) {
        try {
            data = event.data.json()
        } catch (e) {
            // Not JSON, probably just text from DevTools test button
            const text = event.data.text()
            data = { title: "Test Push", body: text }
        }
    }

    const title = data.title || "Default Title Because You Forgot One"
    const options = {
        body: data.body || "Something happened lol",
        // icon: data.icon || "/icon.png", // needs to be absolute path
        // badge: "/badge.png", // small monochrome icon (optional)
        data: data.url || "/", // store URL to open on click
        // tag: 'unique-id', // prevents duplicate notifs (optional)
    }

    event.waitUntil(self.registration.showNotification(title, options))
})

self.addEventListener("notificationclick", (event) => {
    event.notification.close() // Close the notification

    // Open the URL stored in the notification's data
    event.waitUntil(clients.openWindow(event.notification.data))
})

// Optional: Handle notification close event
self.addEventListener("notificationclose", (event) => {
    // We could mimic the green bird that shall not be named because of copyright issues and
    // Act like some clingy ex that just won't let go of you, but instead we will just log it to the console for now
    console.log("Notification was closed", event.notification.data)
})

// Optional: Handle push subscription changes (e.g., when the user revokes permission or the subscription expires)
self.addEventListener("pushsubscriptionchange", (event) => {
    // We could try to resubscribe the user here, but for now we'll just log it to the console
    console.log("Push subscription changed", event)
})

self.addEventListener("message", (event) => {
    console.log("MESSAGE RECEIVED IN SW:", event.data)
})

self.registration.showNotification("Direct SW test", {
    body: "Called from SW console itself",
})
