// Service worker — handles Web Share Target (POST /share)

self.addEventListener('install', () => self.skipWaiting());
self.addEventListener('activate', e => e.waitUntil(self.clients.claim()));

self.addEventListener('fetch', event => {
  const url = new URL(event.request.url);
  if (url.pathname === '/share' && event.request.method === 'POST') {
    event.respondWith(handleShareTarget(event.request));
  }
});

async function handleShareTarget(request) {
  const formData = await request.formData();
  const sharedUrl   = formData.get('url')   || '';
  const sharedText  = formData.get('text')  || '';
  const sharedTitle = formData.get('title') || '';
  const sharedFile  = formData.get('file');

  const params = new URLSearchParams();

  // Case 1 — a photo was shared
  if (sharedFile && sharedFile.size > 0) {
    try {
      const fd = new FormData();
      fd.append('photo', sharedFile);
      const res = await fetch('/api/upload', { method: 'POST', body: fd, credentials: 'include' });
      if (res.ok) {
        const data = await res.json();
        params.set('share_image', data.url);
      } else if (res.status === 401) {
        // Not logged in — can't upload, tell the app
        params.set('share_login_required', '1');
        params.set('share_hint', sharedTitle || 'photo partagée');
      }
    } catch (_) {
      params.set('share_hint', sharedTitle || 'photo partagée');
    }
  }
  // Case 2 — a URL was shared
  else {
    const effectiveUrl = sharedUrl || extractUrl(sharedText);
    if (effectiveUrl) {
      params.set('share_url', effectiveUrl);
    } else if (sharedText) {
      params.set('share_text', sharedText);
    } else if (sharedTitle) {
      params.set('share_text', sharedTitle);
    }
  }

  return Response.redirect('/?' + params.toString(), 303);
}

function extractUrl(text) {
  const match = text.match(/https?:\/\/[^\s]+/);
  return match ? match[0] : '';
}
