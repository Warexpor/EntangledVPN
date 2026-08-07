<script>
  import { status, peers } from '../stores/app.js'
  import { t, fmt } from '../locales/index.js'
  import { onMount } from 'svelte'

  let version = '1.0.0'
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
  }
  .status-left,
  .status-right {
    display: flex;
    align-items: center;
    gap: 8px 14px;
    min-width: 0;
    flex-wrap: wrap;
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
</style>
