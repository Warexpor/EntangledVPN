<script>
  import { status, view, chatOpen, addNotification } from '../stores/app.js'
  import { t, fmt } from '../locales/index.js'
  import ThemeToggle from './ThemeToggle.svelte'
  import ConfirmDialog from './ConfirmDialog.svelte'

  export let pathAggregate = 'disconnected'

  $: pathLive = pathAggregate === 'direct'
  $: pathFault = pathAggregate === 'relay' || pathAggregate === 'reconnecting'
  $: pathIdle = pathAggregate === 'disconnected'
  $: pathLabel = pathAggregate === 'reconnecting'
    ? $t.status_reconnecting
    : pathAggregate === 'relay'
      ? $t.status_relay_degraded
      : pathAggregate === 'direct'
        ? $t.status_direct
        : $t.status_disconnected

  function goHome() {
    view.set('network')
  }

  function goSettings() {
    view.set($view === 'settings' ? 'network' : 'settings')
  }

  async function copyIP() {
    if (!$status.virtualIP) return
    try {
      await window.go.main.App.CopyText($status.virtualIP)
      addNotification($t.copied)
    } catch (_) {}
  }

  let askDisconnect = false

  async function doDisconnect() {
    askDisconnect = false
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
      chatOpen.set(false)
      view.set('connect')
      addNotification($t.notif_disconnected)
    } catch (e) {
      addNotification(fmt($t.error_disconnect, { err: e }), 'error')
    }
  }
</script>

<header class="topbar">
  <div class="topbar-row">
    <button type="button" class="cell room" on:click={goHome} title={$t.network}>
      <span class="label-track">{$t.network}</span>
      <span class="cell-value">{$status.room || $t.network}</span>
    </button>
    <button
      type="button"
      class="cell vip"
      on:click={copyIP}
      disabled={!$status.virtualIP}
      title={$t.virtual_ip}
    >
      <span class="label-track">{$t.virtual_ip}</span>
      <span class="cell-value mono">{$status.virtualIP || $t.na}</span>
    </button>
    <div class="cell path" role="status" aria-live="polite">
      <span class="label-track">{$t.col_path}</span>
      <span class="path-row">
        <span class="leds" aria-hidden="true">
          <span class="led live" class:on={pathLive}></span>
          <span class="led fault" class:on={pathFault}></span>
          <span class="led idle" class:on={pathIdle}></span>
        </span>
        <span class="cell-value" class:live={pathLive} class:fault={pathFault}>{pathLabel}</span>
      </span>
    </div>
    <div class="cell peers">
      <span class="label-track">{$t.peers_list}</span>
      <span class="cell-value mono">{$status.peerCount}</span>
    </div>
    <div class="actions">
      <ThemeToggle chrome />
      <button type="button" class="bezel" class:active={$view === 'settings'} on:click={goSettings}>
        {$t.settings}
      </button>
      <button type="button" class="bezel" on:click={() => (askDisconnect = true)}>
        {$t.disconnect}
      </button>
    </div>
  </div>
</header>

<ConfirmDialog
  open={askDisconnect}
  message={$t.confirm_disconnect}
  confirmLabel={$t.disconnect}
  danger
  onConfirm={doDisconnect}
  onCancel={() => (askDisconnect = false)}
/>

<style>
  .topbar {
    flex-shrink: 0;
    border-bottom: 1px solid var(--border);
    background: var(--surface);
  }
  .topbar-row {
    display: flex;
    align-items: stretch;
    gap: 0;
    min-height: 64px;
  }
  .cell {
    display: flex;
    flex-direction: column;
    justify-content: center;
    gap: 4px;
    padding: 10px 16px;
    border-right: 1px solid var(--border);
    min-width: 0;
    text-align: left;
  }
  button.cell {
    background: none;
    border-top: 0;
    border-bottom: 0;
    border-left: 0;
    cursor: pointer;
    font: inherit;
    color: inherit;
  }
  button.cell:hover .cell-value { color: var(--muted); }
  button.cell:disabled {
    cursor: default;
    opacity: 1;
  }
  button.cell:disabled:hover .cell-value { color: var(--text); }
  .room { flex: 1.4 1 140px; }
  .vip { flex: 1 1 148px; }
  .path { flex: 1.2 1 180px; }
  .peers { flex: 0 0 72px; }
  .cell-value {
    font-size: 1.05rem;
    font-weight: 600;
    letter-spacing: -0.03em;
    color: var(--text);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    line-height: 1.15;
  }
  .cell-value.mono {
    font-family: var(--font-mono);
    font-weight: 500;
    letter-spacing: 0;
    font-size: 1rem;
  }
  .cell-value.live { color: var(--live); }
  .cell-value.fault { color: var(--fault); }
  .path-row {
    display: flex;
    align-items: center;
    gap: 8px;
    min-width: 0;
  }
  .leds {
    display: flex;
    gap: 4px;
    flex-shrink: 0;
  }
  .actions {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 0 12px;
    margin-left: auto;
    flex-shrink: 0;
  }
</style>
