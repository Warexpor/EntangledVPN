/**
 * @typedef {'direct' | 'relay' | 'reconnecting' | 'disconnected'} PathAggregate
 */

/**
 * One room-level path for header and status. Relay or websocket on any
 * live peer is worse than Direct. Reconnect beats both.
 *
 * @param {{ connected?: boolean, reconnecting?: boolean }} status
 * @param {Array<{ connected?: boolean, path?: string }> | null | undefined} peers
 * @returns {PathAggregate}
 */
export function derivePathAggregate(status, peers) {
  if (status?.reconnecting) return 'reconnecting'
  if (!status?.connected) return 'disconnected'
  const list = peers || []
  const relay = list.some(
    (p) => p.connected && (p.path === 'relay' || p.path === 'ws'),
  )
  return relay ? 'relay' : 'direct'
}
