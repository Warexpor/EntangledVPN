import { writable, derived, get } from 'svelte/store'

export const MAX_MESSAGES = 500

export const settings = writable({
  serverAddr: '', nickname: '', autoConnect: false,
  autoJoinLastRoom: false, startWithWindows: false,
  connectionMode: 'direct', mtu: 1500,
  dnsServer: '', socks5Proxy: '', stunServer: 'stun.l.google.com:19302',
  uiScale: 100, theme: 'dark',
  lang: 'en', serverToken: '',
})

export const status = writable({
  connected: false,
  reconnecting: false,
  server: '',
  room: '',
  virtualIP: '',
  peerCount: 0,
  isOwner: false,
  phase: 'idle',
})

export const peers = writable([])
export const view = writable('connect')
export const notifications = writable([])
export const durableError = writable(null)
export const chatThreads = writable({})
export const chatTarget = writable(null)
export const unread = writable({})
export const pendingJoin = writable('')

let notifId = 0
let chatId = 0
let durableId = 0

export function addNotification(msg, type = 'info') {
  const id = ++notifId
  notifications.update(n => [...n, { id, msg, type }])
  setTimeout(() => {
    notifications.update(n => n.filter(x => x.id !== id))
  }, 4000)
  if (type === 'error') {
    setDurableError(msg)
  }
}

export function removeNotification(id) {
  notifications.update(n => n.filter(x => x.id !== id))
}

export function setDurableError(msg) {
  if (!msg) {
    durableError.set(null)
    return
  }
  durableError.set({ id: ++durableId, msg: String(msg) })
}

export function clearDurableError() {
  durableError.set(null)
}

export function threadKey(target, room) {
  if (target && target.id) return 'peer:' + target.id
  return 'room:' + (room || 'network')
}

export function activeThreadKey() {
  return threadKey(get(chatTarget), get(status).room)
}

export function addChatMessage(fromID, nickname, message, isSelf = false, delivery = 'sent', system = false, isDM = false) {
  const id = ++chatId
  let storeKey
  if (system) {
    storeKey = threadKey(null, get(status).room)
  } else if (isSelf) {
    storeKey = activeThreadKey()
  } else if (isDM) {
    storeKey = 'peer:' + fromID
  } else {
    storeKey = threadKey(null, get(status).room)
  }

  chatThreads.update(threads => {
    const list = threads[storeKey] ? [...threads[storeKey]] : []
    list.push({
      id: isSelf && fromID.startsWith('local_') ? fromID : id,
      threadKey: storeKey,
      fromID,
      nickname,
      message,
      timestamp: Date.now(),
      isSelf,
      status: delivery,
      system,
    })
    return {
      ...threads,
      [storeKey]: list.length > MAX_MESSAGES ? list.slice(-MAX_MESSAGES) : list,
    }
  })

  const viewing = get(view)
  const active = activeThreadKey()
  if (!isSelf && !(viewing === 'chat' && storeKey === active)) {
    unread.update(u => ({ ...u, [storeKey]: (u[storeKey] || 0) + 1 }))
  }
}

export function addSystemChat(message) {
  addChatMessage('system', '', message, false, 'sent', true)
}

export function markThreadRead(key) {
  unread.update(u => {
    const n = { ...u }
    delete n[key]
    return n
  })
}

export function updateMessageStatus(localId, delivery, key = activeThreadKey()) {
  chatThreads.update(threads => {
    const list = threads[key]
    if (!list) return threads
    return {
      ...threads,
      [key]: list.map(m => (m.id === localId ? { ...m, status: delivery } : m)),
    }
  })
}

export const chatMessages = derived(
  [chatThreads, chatTarget, status],
  ([$chatThreads, $chatTarget, $status]) => $chatThreads[threadKey($chatTarget, $status.room)] || [],
)
