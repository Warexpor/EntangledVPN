<script>
  import { status, peers, view, chatTarget, settings, unread, markThreadRead, addNotification } from '../stores/app.js'
  import { t, fmt } from '../locales/index.js'

  let pinging = {}

  $: sortedPeers = [...$peers].sort((a, b) => (a.nickname || '').localeCompare(b.nickname || ''))
  $: roomUnread = $unread['room:' + ($status.room || 'network')] || 0

  function openPeerChat(peer) {
    chatTarget.set({ id: peer.id, nickname: peer.nickname || peer.id })
    markThreadRead('peer:' + peer.id)
    view.set('chat')
  }

  function openRoomChat() {
    chatTarget.set(null)
    markThreadRead('room:' + ($status.room || 'network'))
    view.set('chat')
  }

  async function copyIP(ip) {
    if (!ip) return
    try {
      await window.go.main.App.CopyText(ip)
      addNotification($t.copied)
    } catch (_) {}
  }

  async function pingPeer(peer) {
    if (!peer?.id || !peer.connected || pinging[peer.id]) return
    pinging = { ...pinging, [peer.id]: true }
    try {
      await window.go.main.App.PingPeer(peer.id)
      addNotification(fmt($t.ping_sent, { name: peer.nickname || peer.id }))
    } catch (e) {
      addNotification(fmt($t.error_ping, { err: e }), 'error')
    } finally {
      setTimeout(() => {
        const next = { ...pinging }
        delete next[peer.id]
        pinging = next
      }, 800)
    }
  }

  function peerUnread(id) {
    return $unread['peer:' + id] || 0
  }

  function pathLabel(path) {
    if (path === 'p2p') return $t.path_p2p
    if (path === 'relay') return $t.path_relay
    if (path === 'ws') return $t.path_ws
    return $t.path_unknown
  }

  function pathTone(path) {
    if (path === 'p2p') return 'direct'
    if (path === 'relay' || path === 'ws') return 'relay'
    return ''
  }

  function pathTip(path) {
    if (path === 'p2p') return $t.path_p2p_tip
    if (path === 'relay') return $t.path_relay_tip
    if (path === 'ws') return $t.path_ws_tip
    return ''
  }

  function onIpKey(e, ip) {
    if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault()
      copyIP(ip)
    }
  }
</script>

<div class="peer-view">
  <div class="peer-header">
    <h2>{$t.header_peers}</h2>
    {#if $status.room}
      <button class="room-chat-btn" on:click={openRoomChat} title={$t.room_chat}>
        {$t.chat}
        {#if roomUnread > 0}<span class="badge unread-badge">{roomUnread}</span>{/if}
      </button>
    {/if}
  </div>

  <div class="peer-table" role="table" aria-label={$t.networks}>
    <div class="table-row table-header-row" role="row">
      <span class="sq-note" role="columnheader" aria-label={$t.col_status}></span>
      <span class="col-name" role="columnheader">{$t.col_nickname}</span>
      <span class="col-ip" role="columnheader">{$t.col_ip}</span>
      <span class="col-ping" role="columnheader">{$t.col_ping}</span>
      <span class="col-path" role="columnheader">{$t.col_path}</span>
      <span class="col-action" role="columnheader"></span>
    </div>

    <div class="table-row self-row" role="row">
      <span class="status-cell" role="cell">
        <span class="sq sq-online" aria-hidden="true"></span>
        <span class="status-text">{$t.status_online}</span>
      </span>
      <span class="col-name" role="cell">
        <span class="nickname">{$settings.nickname || $t.you}</span>
        <span class="self-tag">{$t.me}</span>
      </span>
      <span
        class="col-ip clickable"
        role="button"
        tabindex="0"
        on:click={() => copyIP($status.virtualIP)}
        on:keydown={(e) => onIpKey(e, $status.virtualIP)}
      >{$status.virtualIP || $t.na}</span>
      <span class="col-ping" role="cell">-</span>
      <span class="col-path" role="cell">-</span>
      <span class="col-action" role="cell"></span>
    </div>

    {#each sortedPeers as peer (peer.id)}
      <div class="table-row" role="row">
        <span class="status-cell" role="cell">
          <span class="sq {peer.connected ? 'sq-online' : 'sq-offline'}" aria-hidden="true"></span>
          <span class="status-text">{peer.connected ? $t.status_online : $t.status_offline}</span>
        </span>
        <span class="col-name" role="cell">
          <span class="nickname">{peer.nickname || peer.id}</span>
          {#if peerUnread(peer.id) > 0}<span class="badge unread-badge">{peerUnread(peer.id)}</span>{/if}
        </span>
        <span
          class="col-ip clickable"
          role="button"
          tabindex="0"
          on:click={() => copyIP(peer.virtualIP)}
          on:keydown={(e) => onIpKey(e, peer.virtualIP)}
        >{peer.virtualIP || $t.na}</span>
        <span class="col-ping" role="cell" class:probing={peer.connected && (peer.ping < 0 || pinging[peer.id])}>
          {#if !peer.connected}
            -
          {:else if peer.ping >= 0 && !pinging[peer.id]}
            {peer.ping}ms
          {:else}
            ...
          {/if}
        </span>
        <span
          class="col-path"
          role="cell"
          title={pathTip(peer.path)}
        >
          {#if peer.path}
            <span class="path-dot {pathTone(peer.path)}" aria-hidden="true"></span>
            {pathLabel(peer.path)}
          {:else}
            {$t.path_unknown}
          {/if}
        </span>
        <span class="col-action" role="cell">
          <button
            class="row-btn"
            on:click={() => pingPeer(peer)}
            disabled={!peer.connected || !!pinging[peer.id]}
            title={$t.ping_peer}
            aria-label={$t.ping_peer}
          >
            <svg width="14" height="14" viewBox="0 0 16 16" aria-hidden="true">
              <circle cx="8" cy="8" r="1.6" fill="currentColor" />
              <circle cx="8" cy="8" r="4.2" fill="none" stroke="currentColor" stroke-width="1.2" />
              <circle cx="8" cy="8" r="6.4" fill="none" stroke="currentColor" stroke-width="1.1" opacity="0.55" />
            </svg>
          </button>
          <button
            class="row-btn"
            on:click={() => openPeerChat(peer)}
            title="{$t.chat} {peer.nickname || peer.id}"
            aria-label="{$t.chat} {peer.nickname || peer.id}"
          >
            <svg width="14" height="14" viewBox="0 0 16 16" aria-hidden="true">
              <path d="M3 4.5h10v6.5H7.2L4 13.5V11H3z" fill="none" stroke="currentColor" stroke-width="1.2" stroke-linejoin="round" />
            </svg>
          </button>
        </span>
      </div>
    {:else}
      <div class="empty-peers">
        <div class="empty-label">{$t.no_peers}</div>
        <div class="empty-desc">{$t.no_peers_desc}</div>
      </div>
    {/each}
  </div>
</div>

<style>
  .peer-view {
    display: flex;
    flex-direction: column;
    height: 100%;
  }
  .peer-header {
    padding: 12px 16px;
    border-bottom: 1px solid var(--border);
    display: flex;
    align-items: center;
    gap: 10px;
    background: var(--bg-surface);
    flex-wrap: wrap;
  }
  .peer-header h2 {
    font-size: 10px;
    font-weight: 600;
    color: var(--text-dim);
    text-transform: uppercase;
    letter-spacing: 0.08em;
  }
  .room-chat-btn {
    margin-left: auto;
    background: transparent;
    border: 1px solid var(--border);
    border-radius: var(--radius-md, 6px);
    color: var(--text-secondary);
    font-family: inherit;
    font-size: var(--font-size-xs);
    padding: 6px 10px;
    min-height: 32px;
    cursor: pointer;
    position: relative;
  }
  .room-chat-btn:hover { color: var(--text-bright); border-color: var(--border-hover); }
  .badge {
    display: inline-block;
    min-width: 14px;
    padding: 0 4px;
    margin-left: 4px;
    background: var(--fault, var(--error));
    color: #fff;
    font-size: 10px;
    line-height: 14px;
    text-align: center;
  }
  .peer-table {
    flex: 1;
    overflow: auto;
    min-height: 0;
    min-width: 0;
  }
  .table-row {
    display: grid;
    grid-template-columns: minmax(56px, 72px) minmax(72px, 1fr) minmax(88px, 110px) minmax(48px, 70px) minmax(72px, 96px) minmax(64px, 80px);
    gap: 8px;
    align-items: center;
    padding: 10px 16px;
    min-height: 40px;
    border-bottom: 1px solid var(--border-deep);
    font-size: var(--font-size-sm);
    min-width: 520px;
  }
  .table-header-row {
    color: var(--text-muted);
    text-transform: uppercase;
    font-size: var(--font-size-xs);
    letter-spacing: 0.08em;
  }
  .self-row { background: var(--bg-surface); }
  .nickname { color: var(--text-bright); min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .self-tag {
    margin-left: 6px;
    font-size: var(--font-size-xs);
    color: var(--text-muted);
  }
  .col-ip, .col-ping, .col-path {
    font-family: var(--font-mono);
    color: var(--text-secondary);
  }
  .col-path {
    display: flex;
    align-items: center;
    gap: 6px;
  }
  .col-ping.probing { color: var(--text-bright); }
  .clickable { cursor: pointer; }
  .clickable:hover { color: var(--text-bright); }
  .status-cell {
    display: flex;
    align-items: center;
    gap: 6px;
  }
  .status-text {
    font-size: var(--font-size-xs);
    color: var(--text-secondary);
  }
  .sq {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    display: inline-block;
    flex-shrink: 0;
  }
  .sq-online { background: var(--text-bright); }
  .sq-offline { background: var(--text-dim); }
  .col-action {
    display: flex;
    gap: 2px;
    justify-content: flex-end;
  }
  .row-btn {
    background: transparent;
    border: none;
    border-radius: var(--radius-sm, 4px);
    color: var(--text-muted);
    cursor: pointer;
    min-width: 32px;
    min-height: 32px;
    padding: 0;
    display: flex;
    align-items: center;
    justify-content: center;
  }
  .row-btn:hover:not(:disabled) { color: var(--text-bright); }
  .row-btn:disabled {
    opacity: 0.35;
    cursor: not-allowed;
  }
  .empty-peers {
    padding: 48px 24px;
    text-align: center;
    color: var(--text-muted);
  }
  .empty-label {
    color: var(--text-secondary);
    margin: 8px 0 4px;
  }
  .empty-desc { font-size: var(--font-size-xs); }
</style>
