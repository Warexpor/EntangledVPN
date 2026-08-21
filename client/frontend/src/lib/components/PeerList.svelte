<script>
  import { status, peers, chatOpen, chatTarget, settings, unread, markThreadRead, addNotification } from '../stores/app.js'
  import { t, fmt } from '../locales/index.js'

  export let pathAggregate = 'disconnected'

  let pinging = {}

  $: sortedPeers = [...$peers].sort((a, b) => (a.nickname || '').localeCompare(b.nickname || ''))
  $: roomUnread = $unread['room:' + ($status.room || 'network')] || 0

  function openPeerChat(peer) {
    chatTarget.set({ id: peer.id, nickname: peer.nickname || peer.id })
    markThreadRead('peer:' + peer.id)
    chatOpen.set(true)
  }

  function openRoomChat() {
    chatTarget.set(null)
    markThreadRead('room:' + ($status.room || 'network'))
    chatOpen.set(true)
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

  function pathKind(path) {
    if (path === 'p2p') return 'live'
    if (path === 'relay' || path === 'ws') return 'fault'
    return 'idle'
  }

  function peerDot(peer) {
    if (!peer.connected) return 'idle'
    return pathKind(peer.path)
  }

  function chIndex(i) {
    return String(i).padStart(2, '0')
  }
</script>

<div class="peer-view">
  <div class="section-head">
    <div>
      <span class="label-track">{$t.channels}</span>
      <h2>{$status.room || $t.peers_list}</h2>
    </div>
    {#if $status.room}
      <button type="button" class="bezel" on:click={openRoomChat}>
        {$t.room_chat}
        {#if roomUnread > 0}<span class="badge">{roomUnread}</span>{/if}
      </button>
    {/if}
  </div>
  {#if pathAggregate === 'relay'}
    <p class="section-hint fault">{$t.status_relay_degraded}</p>
  {/if}

  <div class="scope-face">
    <div class="channel-head" aria-hidden="true">
      <span>{$t.col_ch}</span>
      <span>{$t.col_nickname}</span>
      <span>{$t.col_ip}</span>
      <span>{$t.col_path}</span>
      <span>{$t.col_ping}</span>
      <span></span>
    </div>
    <div class="stack" role="list" aria-label={$t.peers_list}>
      <article class="peer-row self-row" role="listitem">
        <span class="ch">{chIndex(0)}</span>
        <div class="nickname">
          {$settings.nickname || $t.you}
          <span class="self-tag">{$t.me}</span>
        </div>
        <button
          type="button"
          class="peer-ip"
          on:click={() => copyIP($status.virtualIP)}
          on:keydown={(e) => onIpKey(e, $status.virtualIP)}
        >{$status.virtualIP || $t.na}</button>
        <span class="col-path idle">{$t.path_unknown}</span>
        <span class="col-ping">—</span>
        <span class="peer-actions"></span>
      </article>

      {#each sortedPeers as peer, i (peer.id)}
        {@const kind = peerDot(peer)}
        <article class="peer-row" role="listitem">
          <span class="ch">{chIndex(i + 1)}</span>
          <div class="nickname">
            {peer.nickname || peer.id}
            {#if peerUnread(peer.id) > 0}<span class="badge">{peerUnread(peer.id)}</span>{/if}
          </div>
          <button
            type="button"
            class="peer-ip"
            on:click={() => copyIP(peer.virtualIP)}
            on:keydown={(e) => onIpKey(e, peer.virtualIP)}
          >{peer.virtualIP || $t.na}</button>
          <span
            class="col-path"
            class:live={kind === 'live'}
            class:fault={kind === 'fault'}
            class:idle={kind === 'idle'}
            title={pathTip(peer.path)}
          >
            <svg class="trace" viewBox="0 0 40 10" aria-hidden="true">
              <line x1="3" y1="5" x2="37" y2="5" />
              <circle cx="5" cy="5" r="2" />
              <circle cx="35" cy="5" r="2" />
            </svg>
            {peer.connected && peer.path ? pathLabel(peer.path) : $t.path_unknown}
          </span>
          <span class="col-ping" class:probing={peer.connected && (peer.ping < 0 || pinging[peer.id])}>
            {#if !peer.connected}
              —
            {:else if peer.ping >= 0 && !pinging[peer.id]}
              {peer.ping}ms
            {:else}
              …
            {/if}
          </span>
          <div class="peer-actions">
            <button
              class="btn btn-ghost btn-sm"
              on:click={() => pingPeer(peer)}
              disabled={!peer.connected || !!pinging[peer.id]}
            >{$t.ping_peer}</button>
            <button class="btn btn-ghost btn-sm" on:click={() => openPeerChat(peer)}>
              {$t.chat}
            </button>
          </div>
        </article>
      {:else}
        <div class="empty">
          <div class="empty-label">{$t.no_peers}</div>
          <p class="empty-desc">{$t.no_peers_desc}</p>
        </div>
      {/each}
    </div>
  </div>
</div>

<style>
  .peer-view {
    display: flex;
    flex-direction: column;
    height: 100%;
    padding: 12px 16px 20px;
    overflow: auto;
    min-height: 0;
  }
  .section-head {
    display: flex;
    align-items: flex-end;
    justify-content: space-between;
    gap: 12px;
    margin-bottom: 10px;
  }
  .section-head h2 {
    font-size: 1.05rem;
    font-weight: 600;
    letter-spacing: -0.03em;
    color: var(--text);
    margin: 2px 0 0;
  }
  .section-hint {
    color: var(--muted);
    font-size: 13px;
    margin: 0 0 10px;
  }
  .section-hint.fault { color: var(--fault); }
  .scope-face {
    position: relative;
    flex: 1;
    min-height: 0;
    border: 1px solid var(--border);
    background-color: var(--bg);
    background-image:
      repeating-linear-gradient(
        to right,
        var(--graticule) 0 1px,
        transparent 1px 48px
      ),
      repeating-linear-gradient(
        to bottom,
        var(--graticule) 0 1px,
        transparent 1px 28px
      );
  }
  .channel-head,
  .peer-row {
    display: grid;
    grid-template-columns: 36px minmax(0, 1.3fr) 118px 128px 56px auto;
    gap: 10px;
    align-items: center;
    padding: 0 12px;
  }
  .channel-head {
    min-height: 28px;
    border-bottom: 1px solid var(--border);
    background: color-mix(in srgb, var(--surface) 82%, transparent);
    font-size: 10px;
    font-weight: 500;
    letter-spacing: 0.12em;
    text-transform: uppercase;
    color: var(--muted);
  }
  .stack {
    display: flex;
    flex-direction: column;
  }
  .peer-row {
    min-height: 44px;
    border-bottom: 1px solid var(--border);
    background: color-mix(in srgb, var(--bg) 55%, transparent);
  }
  .self-row { opacity: 0.88; }
  .ch {
    font-family: var(--font-mono);
    font-size: 11px;
    color: var(--muted);
  }
  .nickname {
    color: var(--text);
    font-weight: 500;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .self-tag {
    margin-left: 6px;
    font-size: 11px;
    color: var(--muted);
    font-weight: 400;
    letter-spacing: 0.06em;
    text-transform: uppercase;
  }
  .peer-ip {
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--muted);
    background: none;
    border: 0;
    padding: 0;
    cursor: pointer;
    text-align: left;
  }
  .peer-ip:hover { color: var(--text); }
  .col-ping {
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--muted);
  }
  .col-ping.probing { color: var(--text); }
  .col-path {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    font-size: 12px;
    font-weight: 500;
    font-family: var(--font-mono);
    color: var(--muted);
    min-width: 0;
  }
  .col-path.live { color: var(--live); }
  .col-path.fault { color: var(--fault); }
  .col-path.idle { color: var(--muted); }
  .trace {
    width: 36px;
    height: 10px;
    flex-shrink: 0;
  }
  .trace line,
  .trace circle {
    fill: currentColor;
    stroke: currentColor;
    stroke-width: 1;
  }
  .peer-actions {
    display: flex;
    gap: 6px;
    justify-content: flex-end;
  }
  .badge {
    display: inline-block;
    min-width: 16px;
    padding: 0 5px;
    margin-left: 6px;
    background: var(--fault);
    color: #fff;
    font-size: 10px;
    line-height: 16px;
    text-align: center;
    border-radius: var(--radius);
  }
  .empty {
    padding: 20px 12px;
    color: var(--muted);
    background: color-mix(in srgb, var(--bg) 70%, transparent);
  }
  .empty-label { color: var(--text); margin-bottom: 4px; font-weight: 500; }
  .empty-desc {
    font-family: var(--font-serif);
    font-style: italic;
    font-size: 13px;
    margin: 0;
  }
  .btn-sm {
    min-height: 28px;
    padding: 4px 10px;
    font-size: 12px;
  }
  @media (max-width: 720px) {
    .channel-head, .peer-row { grid-template-columns: 28px minmax(0, 1fr) auto; }
    .channel-head span:nth-child(3),
    .channel-head span:nth-child(4),
    .channel-head span:nth-child(5),
    .peer-ip, .col-path, .col-ping { display: none; }
  }
</style>
