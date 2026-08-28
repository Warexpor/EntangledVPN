<script>
  import { status, peers, view, addNotification } from '../stores/app.js'
  import { t, fmt } from '../locales/index.js'
  import { onMount } from 'svelte'
  import ConfirmDialog from './ConfirmDialog.svelte'

  let version = '1.0.0'
  let showDisconnectConfirm = false
  let disconnecting = false
  onMount(async () => {
    try {
      if (window.go?.main?.App?.GetVersion) {
        version = await window.go.main.App.GetVersion()
      }
    } catch (_) {}
  })

  $: anyRelay = ($peers || []).some(p => p.path === 'relay' || p.path === 'ws')
  $: statusLabel = $status.reconnecting
    ? $t.status_reconnecting
    : ($status.connected ? $t.status_connected : $t.status_disconnected)
  $: statusTip = anyRelay && $status.connected
    ? $t.status_relay_degraded
    : statusLabel

  function disconnect() {
    showDisconnectConfirm = true
  }

  async function executeDisconnect() {
    showDisconnectConfirm = false
    disconnecting = true
    localStorage.setItem('entangled_manual_disconnect', '1')
    try {
      await window.go.main.App.Disconnect()
      status.set({ connected: false, reconnecting: false, server: '', room: '', virtualIP: '', peerCount: 0, isOwner: false, phase: 'idle' })
      view.set('connect')
      addNotification($t.notif_disconnected)
    } catch (e) {
      addNotification(fmt($t.error_disconnect, { err: e }), 'error')
    } finally {
      disconnecting = false
    }
  }
</script>

<footer class="statusbar" role="status" aria-live="polite">
  <div class="status-left">
    <span
      class="sq"
      class:sq-connected={$status.connected && !$status.reconnecting}
      class:sq-warn={$status.reconnecting}
      aria-hidden="true"
    ></span>
    <span class="status-item" title={statusTip}>{statusLabel}</span>
    {#if anyRelay && $status.connected}
      <span class="degraded status-item" title={$t.path_relay_tip}>{$t.status_relay_degraded}</span>
    {/if}
  </div>
  <div class="status-right">
    {#if $status.server}
      <span class="status-item status-server" title={$status.server}>{fmt($t.status_server, { s: $status.server })}</span>
    {/if}
    {#if $status.virtualIP}
      <span class="status-item" title={$status.virtualIP}>{fmt($t.status_ip, { ip: $status.virtualIP })}</span>
    {/if}
    {#if $status.room}
      <span class="status-item status-room" title={$status.room}>{fmt($t.status_network, { room: $status.room })}</span>
    {/if}
    {#if $status.peerCount !== undefined}
      <span class="status-item">{fmt($t.status_peers, { n: $status.peerCount })}</span>
    {/if}
    {#if $status.connected || $status.reconnecting}
      <button type="button" class="disconnect-btn" on:click={disconnect} disabled={disconnecting}>
        {disconnecting ? $t.disconnecting : $t.disconnect}
      </button>
    {/if}
    <span class="status-item status-ver">{fmt($t.version, { v: version })}</span>
  </div>
</footer>

<style>
  .statusbar {
    min-height: var(--statusbar-height);
    background: var(--bg-deep);
    border-top: 1px solid var(--border);
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    padding: 4px 12px;
    font-size: var(--font-size-xs);
    color: var(--text-secondary);
    flex-shrink: 0;
    font-family: var(--font-mono);
    z-index: 1;
    position: relative;
    min-width: 0;
    overflow: hidden;
  }
  .status-left,
  .status-right {
    display: flex;
    align-items: center;
    gap: 8px 14px;
    min-width: 0;
    flex-wrap: nowrap;
  }
  .status-left {
    flex: 0 1 auto;
  }
  .status-right {
    flex: 1 1 auto;
    justify-content: flex-end;
    opacity: 0.85;
  }
  .status-item {
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    max-width: 36ch;
  }
  .status-server { max-width: 28ch; }
  .status-room { max-width: 22ch; }
  .status-ver { flex-shrink: 0; max-width: none; }
  .disconnect-btn {
    flex-shrink: 0;
    min-height: 24px;
    padding: 2px 8px;
    border: 1px solid var(--error);
    color: var(--error);
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }
  .disconnect-btn:hover:not(:disabled) {
    background: rgba(226, 61, 61, 0.1);
  }
  .degraded { color: var(--warning); }
  .sq {
    display: inline-block;
    width: 7px;
    height: 7px;
    background: var(--error);
    vertical-align: middle;
    flex-shrink: 0;
  }
  .sq.sq-connected { background: var(--success); }
  .sq.sq-warn { background: var(--warning); }
  @media (max-width: 760px) {
    .status-right { gap: 8px; }
    .status-server, .status-room, .status-ver { display: none; }
  }
</style>

{#if showDisconnectConfirm}
  <ConfirmDialog
    open={true}
    message={$t.confirm_disconnect}
    danger={true}
    on:cancel={() => (showDisconnectConfirm = false)}
    on:confirm={executeDisconnect}
  />
{/if}
