<script>
  import { status } from '../stores/app.js'
  import { t, fmt } from '../locales/index.js'
  import { onMount } from 'svelte'

  export let pathAggregate = 'disconnected'

  let version = '1.0.0'
  onMount(async () => {
    try {
      if (window.go?.main?.App?.GetVersion) {
        version = await window.go.main.App.GetVersion()
      }
    } catch (_) {}
  })
</script>

<footer class="statusbar" role="status" aria-live="polite">
  <div class="status-left">
    {#if pathAggregate === 'reconnecting'}
      <span class="hint-fault">{$t.status_reconnecting}</span>
    {/if}
  </div>
  <div class="status-right">
    {#if $status.server}
      <span class="status-item status-server" title={$status.server}>{fmt($t.status_server, { s: $status.server })}</span>
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
  }
  .status-right {
    flex: 1 1 auto;
    justify-content: flex-end;
  }
  .status-item {
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    max-width: 36ch;
  }
  .status-server { max-width: 28ch; }
  .status-ver { flex-shrink: 0; max-width: none; }
  .hint-fault { color: var(--fault, var(--error)); }
</style>
