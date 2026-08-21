<script>
  import { status, view, addNotification } from '../stores/app.js'
  import { t, fmt } from '../locales/index.js'

  export let pathAggregate = 'disconnected'

  let prevView = 'network'

  $: pathLabel =
    pathAggregate === 'direct' ? $t.path_p2p
    : pathAggregate === 'relay' ? $t.path_relay
    : pathAggregate === 'reconnecting' ? $t.status_reconnecting
    : $t.status_disconnected

  function toggleSettings() {
    if ($view === 'settings') {
      view.set(prevView || 'network')
    } else {
      prevView = $view
      view.set('settings')
    }
  }

  async function copyVip() {
    const ip = $status.virtualIP
    if (!ip) return
    try {
      await window.go.main.App.CopyText(ip)
      addNotification($t.copied)
    } catch (_) {}
  }

  async function disconnect() {
    if (!confirm($t.confirm_disconnect)) return
    try {
      await window.go.main.App.Disconnect()
      status.set({
        connected: false,
        reconnecting: false,
        server: '',
        room: '',
        virtualIP: '',
        peerCount: 0,
        isOwner: false,
        phase: 'idle',
      })
      view.set('connect')
      addNotification($t.notif_disconnected)
    } catch (e) {
      addNotification(fmt($t.error_disconnect, { err: e }), 'error')
    }
  }
</script>

<header class="band">
  <span class="wordmark">{$t.brand}</span>

  <div class="facts">
    <div class="fact">
      <span class="cap">{$t.header_room}</span>
      <span class="val">{$status.room || $t.na}</span>
    </div>
    <div class="fact">
      <span class="cap">{$t.header_vip}</span>
      <button type="button" class="val mono vip" on:click={copyVip} title={$t.virtual_ip}>
        {$status.virtualIP || $t.na}
      </button>
    </div>
    <div class="fact path-fact">
      <span class="cap">{$t.header_path}</span>
      <span class="path-row">
        <span class="path-dot {pathAggregate}" aria-hidden="true"></span>
        <span class="val mono">{pathLabel}</span>
      </span>
    </div>
    <div class="fact">
      <span class="cap">{$t.header_peers}</span>
      <span class="val">{fmt($t.peer_count, { n: $status.peerCount ?? 0 })}</span>
    </div>
  </div>

  <div class="actions">
    <button
      type="button"
      class="btn"
      class:active={$view === 'settings'}
      on:click={toggleSettings}
    >{$t.settings}</button>
    <button type="button" class="btn danger" on:click={disconnect}>{$t.disconnect}</button>
  </div>
</header>

<style>
  .band {
    display: flex;
    align-items: center;
    gap: 16px;
    padding: 8px 12px;
    min-height: 48px;
    background: var(--bg-surface);
    border-bottom: 1px solid var(--border);
    flex-shrink: 0;
  }
  .wordmark {
    font-size: 13px;
    font-weight: 600;
    letter-spacing: 0.04em;
    color: var(--text-bright);
    flex-shrink: 0;
  }
  .facts {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 16px 20px;
    flex: 1;
    min-width: 0;
  }
  .fact {
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-width: 0;
  }
  .cap {
    font-size: 10px;
    font-weight: 600;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--text-dim);
  }
  .val {
    color: var(--text-bright);
    font-size: 12px;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    max-width: 28ch;
  }
  .mono {
    font-family: var(--font-mono);
    font-weight: 500;
  }
  .vip {
    background: none;
    border: none;
    padding: 0;
    text-align: left;
    cursor: pointer;
    color: var(--text-bright);
  }
  .vip:hover {
    text-decoration: underline;
  }
  .path-row {
    display: flex;
    align-items: center;
    gap: 6px;
  }
  .actions {
    display: flex;
    gap: 8px;
    flex-shrink: 0;
    margin-left: auto;
  }
  .btn.active {
    border-color: var(--border-hover);
    background: var(--bg-hover);
    color: var(--text-bright);
  }
</style>
