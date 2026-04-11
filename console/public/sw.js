self.addEventListener('push', function(event) {
  if (!event.data) return;
  
  try {
    const data = event.data.json();
    const title = data.title || 'Notification';
    const options = {
      body: data.body,
      icon: data.icon,
      badge: data.badge,
      image: data.image,
      data: data.data || {}
    };
    
    event.waitUntil(
      self.registration.showNotification(title, options)
    );
  } catch (e) {
    // If not JSON, show simple text
    event.waitUntil(
      self.registration.showNotification('Notification', {
        body: event.data.text()
      })
    );
  }
});

self.addEventListener('notificationclick', function(event) {
  event.notification.close();
  // We can handle click events here, like opening a specific URL
  if (event.notification.data && event.notification.data.url) {
    event.waitUntil(
      clients.openWindow(event.notification.data.url)
    );
  }
});
