<script>
  import { onMount, onDestroy } from 'svelte'
  import { status, peers, view, settings, notifications, durableError, clearDurableError, pendingJoin, addNotification, removeNotification, addChatMessage, addSystemChat } from './lib/stores/app.js'
  import { currentLang, setLang, t, fmt } from './lib/locales/index.js'
  import Sidebar from './lib/components/Sidebar.svelte'
  import PeerList from './lib/components/PeerList.svelte'
  import StatusBar from './lib/components/StatusBar.svelte'
  import ConnectView from './lib/components/ConnectView.svelte'
  import ChatView from './lib/components/ChatView.svelte'
  import SettingsView from './lib/components/SettingsView.svelte'

  function clampScale(n) {
    const v = Number(n) || 100
    return Math.min(150, Math.max(75, v))
  }

  $: {
    const scale = clampScale($settings.uiScale) / 100
    document.documentElement.style.setProperty('--ui-scale', String(scale))
  }

  $: {
    if ($settings.theme === 'light') {
      document.documentElement.classList.add('light-theme')
    } else {
      document.documentElement.classList.remove('light-theme')
    }
  }

  let sidebarWidth = 220
  let isDragging = false
  let startX, startWidth
  let mounted = true

  function onDragStart(e) {
    isDragging = true
    startX = e.clientX
    startWidth = sidebarWidth
    document.addEventListener('mousemove', onDragMove)
    document.addEventListener('mouseup', onDragEnd)
    e.preventDefault()
  }

  function onDragMove(e) {
    if (!isDragging) return
    const newWidth = startWidth + (e.clientX - startX)
    sidebarWidth = Math.max(140, Math.min(400, newWidth))
  }

  function onDragEnd() {
    isDragging = false
    document.removeEventListener('mousemove', onDragMove)
    document.removeEventListener('mouseup', onDragEnd)
    localStorage.setItem('entangled_sidebar_width', String(sidebarWidth))
  }

  function onDragKeydown(e) {
    if (e.key === 'ArrowLeft' || e.key === 'ArrowRight') {
      e.preventDefault()
      const delta = e.key === 'ArrowRight' ? 16 : -16
      sidebarWidth = Math.max(180, Math.min(400, sidebarWidth + delta))
      localStorage.setItem('entangled_sidebar_width', String(sidebarWidth))
    }
  }

  async function onKeyDown(e) {
    if (e.key !== 'F11') return
    e.preventDefault()
    try {
      const fs = await window.runtime?.WindowIsFullscreen?.()
      if (fs) window.runtime.WindowUnfullscreen()
      else window.runtime.WindowFullscreen()
    } catch (_) {}
  }

  onMount(async () => {
    window.addEventListener('keydown', onKeyDown)
    const storedSidebarWidth = Number(localStorage.getItem('entangled_sidebar_width'))
    if (storedSidebarWidth) sidebarWidth = Math.max(180, Math.min(400, storedSidebarWidth))
    try {
      const saved = await window.go.main.App.LoadConfig()
      if (saved) {
        settings.set({ ...$settings, ...saved })
        if (saved.lang) setLang(saved.lang)
      }
    } catch (_) {}

    try {
      if (window.go?.main?.App?.GetStatus) {
        const initialStatus = await window.go.main.App.GetStatus()
        status.set(initialStatus)
        if (initialStatus?.connected) view.set('network')
      }
    } catch (_) {}

    if (window.runtime?.EventsOn) {
      window.runtime.EventsOn("status_changed", (data) => {
        if (!mounted) return
        status.set(data)
        if (!data.connected && !data.reconnecting) {
          view.set('connect')
        }
      })
      window.runtime.EventsOn("peers_changed", (data) => {
        if (!mounted) return
        peers.set(data || [])
      })
      window.runtime.EventsOn("error", (msg) => {
        if (!mounted) return
        addNotification(msg, 'error')
      })
      window.runtime.EventsOn("chat_message", (data) => {
        if (!mounted) return
        addChatMessage(data.fromID, data.nickname, data.message, false, 'sent', false, !!data.isDM)
      })
      window.runtime.EventsOn("system_chat", (data) => {
        if (!mounted) return
        addSystemChat(data.message || '')
      })
      window.runtime.EventsOn("room_deleted", (data) => {
        if (!mounted) return
        addNotification($t.notif_room_deleted, 'error')
        view.set('network')
      })
      window.runtime.EventsOn("auto_join_skipped", (data) => {
        if (!mounted) return
        const room = data?.room || ''
        pendingJoin.set(room)
        addNotification(fmt($t.notif_auto_join_skipped, { name: room }), 'info')
      })
    }
  })

  onDestroy(() => {
    mounted = false
    window.removeEventListener('keydown', onKeyDown)
    document.removeEventListener('mousemove', onDragMove)
    document.removeEventListener('mouseup', onDragEnd)
    if (window.runtime?.EventsOff) {
      window.runtime.EventsOff("status_changed")
      window.runtime.EventsOff("peers_changed")
      window.runtime.EventsOff("error")
      window.runtime.EventsOff("chat_message")
      window.runtime.EventsOff("system_chat")
      window.runtime.EventsOff("room_deleted")
      window.runtime.EventsOff("auto_join_skipped")
    }
  })
</script>

<div class="space-bg" aria-hidden="true"></div>
<div class="space-dust" aria-hidden="true"></div>

{#if $view === 'connect'}
  <div class="shell connect-shell">
    <div class="fullscreen-connect">
      <ConnectView />
    </div>
  </div>
{:else}
  <div class="shell">
    <div class="layout">
      <div class="sidebar-wrapper" style="width: {sidebarWidth}px">
        <Sidebar />
        <button
          type="button"
          class="drag-handle"
          class:active={isDragging}
          role="slider"
          aria-orientation="vertical"
          aria-label={$t.resize_sidebar}
          aria-valuemin="180"
          aria-valuemax="400"
          aria-valuenow={sidebarWidth}
          tabindex="0"
          on:mousedown={onDragStart}
          on:keydown={onDragKeydown}
        ></button>
      </div>

      <main class="main-area">
        {#if $status.reconnecting}
          <div class="reconnect-banner" role="status">
            <span class="sq sq-warn" aria-hidden="true"></span>
            <span>{$t.reconnecting}</span>
            <span class="reconnect-detail">{$t.reconnect_in_progress}</span>
          </div>
        {/if}
        {#key $view}
          <div class="view-fade">
            {#if $view === 'network'}
              <PeerList />
            {:else if $view === 'chat'}
              <ChatView />
            {:else if $view === 'settings'}
              <SettingsView />
            {:else}
              <div class="unknown-view">Unknown view: {$view}</div>
            {/if}
          </div>
        {/key}
      </main>
    </div>

    <StatusBar />
  </div>
{/if}

{#if $durableError}
  <div class="error-strip" role="alert">
    <span class="error-strip-msg">{$durableError.msg}</span>
    <button type="button" class="error-strip-dismiss" on:click={clearDurableError} aria-label={$t.dismiss_error}>
      {$t.dismiss_error}
    </button>
  </div>
{/if}

{#if $notifications.length}
  <div class="notification-stack" aria-live="polite">
    {#each $notifications as notif (notif.id)}
      <div class="notification {notif.type}" role={notif.type === 'error' ? 'alert' : 'status'}>
        <span class="notif-icon" aria-hidden="true">{notif.type === 'error' ? '!' : '>'}</span>
        <span class="notification-message">{notif.msg}</span>
        <button type="button" class="notification-dismiss" on:click={() => removeNotification(notif.id)} aria-label={$t.dismiss}>
          ×
        </button>
      </div>
    {/each}
  </div>
{/if}

<style>
  .shell {
    display: flex;
    flex-direction: column;
    height: 100%;
    width: 100%;
    position: relative;
    z-index: 1;
    min-height: 0;
  }
  .connect-shell {
    z-index: 1;
  }
  .layout {
    display: flex;
    flex: 1;
    min-height: 0;
    position: relative;
  }
  .fullscreen-connect {
    display: flex;
    flex: 1;
    width: 100%;
    align-items: center;
    justify-content: center;
    min-height: 0;
  }
  .sidebar-wrapper {
    position: relative;
    flex-shrink: 0;
    max-width: 38vw;
    background: var(--bg-surface);
    border-right: 1px solid var(--border);
    display: flex;
    flex-direction: column;
    overflow: hidden;
    min-height: 0;
  }
  .drag-handle {
    position: absolute;
    top: 0; right: -3px;
    width: 6px;
    height: 100%;
    cursor: col-resize;
    z-index: var(--z-drag);
    background: transparent;
    transition: background 0.1s;
  }
  .drag-handle:hover, .drag-handle.active, .drag-handle:focus-visible {
    background: var(--border-hover);
  }
  .main-area {
    flex: 1;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    background: var(--bg-primary);
    min-width: 0;
    min-height: 0;
  }
  .view-fade {
    flex: 1;
    display: flex;
    flex-direction: column;
    min-height: 0;
    min-width: 0;
    animation: viewIn 0.2s ease-out;
  }
  @keyframes viewIn {
    0% { opacity: 0; transform: translateY(8px); }
    100% { opacity: 1; transform: translateY(0); }
  }
  .unknown-view {
    padding: 40px;
    color: var(--text-muted);
  }
  .error-strip {
    position: fixed;
    left: 12px;
    right: 12px;
    bottom: calc(var(--statusbar-height) + 12px);
    z-index: var(--z-alert);
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 10px 14px;
    background: var(--bg-raised);
    border: 1px solid var(--error);
    color: var(--error);
    font-size: var(--font-size-sm);
    animation: slideUp 0.18s ease-out;
  }
  .error-strip-msg {
    flex: 1;
    min-width: 0;
    word-break: break-word;
  }
  .error-strip-dismiss {
    flex-shrink: 0;
    min-height: 32px;
    padding: 4px 10px;
    border: 1px solid var(--error);
    background: transparent;
    color: var(--error);
    cursor: pointer;
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    text-transform: uppercase;
  }
  .error-strip-dismiss:hover {
    background: var(--error-surface);
  }
  .notification-stack {
    position: fixed;
    top: 12px;
    right: 12px;
    z-index: var(--z-toast);
    display: flex;
    flex-direction: column;
    align-items: flex-end;
    gap: 8px;
    width: min(360px, calc(100vw - 24px));
    pointer-events: none;
  }
  .notification {
    width: 100%;
    display: flex;
    align-items: flex-start;
    gap: 8px;
    padding: 8px 10px;
    font-size: var(--font-size-sm);
    background: var(--bg-raised);
    border: 1px solid var(--border);
    color: var(--text-primary);
    animation: slideIn 0.15s ease-out;
    pointer-events: auto;
  }
  .notification.error {
    border-color: var(--error);
    color: var(--error);
  }
  .notification.info {
    border-color: var(--border);
  }
  .notif-icon {
    font-weight: 700;
    flex-shrink: 0;
  }
  .notification-message {
    flex: 1;
    min-width: 0;
    overflow-wrap: anywhere;
  }
  .notification-dismiss {
    color: var(--text-muted);
    font-size: 16px;
    line-height: 1;
    padding: 0 2px;
    min-width: 24px;
    min-height: 24px;
  }
  .notification-dismiss:hover {
    color: var(--text-bright);
  }
  @keyframes slideIn {
    from { transform: translateX(100%); opacity: 0; }
    to { transform: translateX(0); opacity: 1; }
  }
  @keyframes slideUp {
    from { opacity: 0; transform: translateY(6px); }
    to { opacity: 1; transform: translateY(0); }
  }
  .reconnect-banner {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 8px 16px;
    border-bottom: 1px solid var(--warning);
    background: var(--warning-surface);
    color: var(--warning);
    font-size: var(--font-size-sm);
  }
  .reconnect-detail {
    color: var(--text-secondary);
  }
  .sq {
    width: 8px;
    height: 8px;
    display: inline-block;
    flex-shrink: 0;
  }
  .sq-warn { background: var(--warning); }
  @media (max-width: 640px) {
    .reconnect-detail { display: none; }
    .error-strip { left: 8px; right: 8px; }
  }
  @media (prefers-reduced-motion: reduce) {
    .view-fade, .notification, .error-strip { animation: none; }
  }
</style>
