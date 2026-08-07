<script>
  import { onMount, onDestroy } from 'svelte'
  import { status, peers, view, settings, notifications, durableError, clearDurableError, addNotification, addChatMessage, addSystemChat } from './lib/stores/app.js'
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
    document.documentElement.style.zoom = String(scale)
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
    try {
      const saved = await window.go.main.App.LoadConfig()
      if (saved) {
        settings.set({ ...$settings, ...saved })
        if (saved.lang) setLang(saved.lang)
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
        <!-- svelte-ignore a11y-no-static-element-interactions -->
        <div
          class="drag-handle"
          class:active={isDragging}
          on:mousedown={onDragStart}
        ></div>
      </div>

      <main class="main-area">
        <div class="view-fade" key={$view}>
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

{#each $notifications as notif (notif.id)}
  <div class="notification {notif.type}" role="status">
    <span class="notif-icon">{notif.type === 'error' ? '!' : '>'}</span>
    {notif.msg}
  </div>
{/each}

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
    z-index: 10;
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
    z-index: 1100;
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 10px 14px;
    background: var(--bg-raised);
    border: 1px solid var(--error);
    color: var(--error);
    font-size: var(--font-size-sm);
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
    background: rgba(196, 92, 92, 0.12);
  }
  .notification {
    position: fixed;
    top: 12px;
    right: 12px;
    padding: 6px 12px;
    font-size: var(--font-size-sm);
    z-index: 1000;
    background: var(--bg-raised);
    border: 1px solid var(--border);
    color: var(--text-primary);
    animation: slideIn 0.15s ease-out;
  }
  .notification.error {
    border-color: var(--error);
    color: var(--error);
  }
  .notification.info {
    border-color: var(--border);
  }
  .notif-icon {
    margin-right: 6px;
    font-weight: 700;
  }
  @keyframes slideIn {
    from { transform: translateX(100%); opacity: 0; }
    to { transform: translateX(0); opacity: 1; }
  }
</style>
