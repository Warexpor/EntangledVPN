<script>
  import { onMount, onDestroy } from 'svelte'
  import { status, peers, view, settings, notifications, durableError, clearDurableError, addNotification, addChatMessage, addSystemChat, chatOpen } from './lib/stores/app.js'
  import { setLang, t, fmt } from './lib/locales/index.js'
  import Sidebar from './lib/components/Sidebar.svelte'
  import PeerList from './lib/components/PeerList.svelte'
  import Topbar from './lib/components/Topbar.svelte'
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
    const root = document.documentElement
    const light = $settings.theme === 'light'
    root.classList.toggle('light-theme', light)
    root.dataset.theme = light ? 'light' : 'dark'
    root.dataset.app = $view === 'connect' ? 'connect' : 'connected'
  }

  let sidebarWidth = 220
  let isDragging = false
  let startX, startWidth
  let mounted = true

  function derivePathAggregate(st, list) {
    if (st.reconnecting) return 'reconnecting'
    if (!st.connected) return 'disconnected'
    const relay = (list || []).some((p) => p.connected && (p.path === 'relay' || p.path === 'ws'))
    return relay ? 'relay' : 'direct'
  }

  $: pathAggregate = derivePathAggregate($status, $peers)

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

{#if $view === 'connect'}
  <div class="space-bg" aria-hidden="true"></div>
  <div class="space-dust" aria-hidden="true"></div>
  <div class="shell connect-shell">
    <div class="fullscreen-connect">
      <ConnectView />
    </div>
  </div>
{:else}
  <div class="shell bench">
    <!--
      THESIS: Connected shell is a signal bench. Path quality is the trigger, not a Discord three-pane reskin.
      OWN-WORLD: Matte B&W panels, 1px hairlines, tracked labels over mono values. --live green only for Direct/p2p. --fault red only for relay, unread, error. No wash or grain.
      STORY: You are in a room with a copyable VIP. Peers are channels. Direct vs relay reads first.
      FIRST VIEWPORT: Readout band (room, VIP, path LEDs, theme, settings, disconnect). Quiet rail left. Graticule channels center. Chat as right readout. Settings in the same bezel language.
      FORM: Oscilloscope signal-bench fused with PRODUCT B&W. Seed 82f5fa76. Comp A topology. User B&W steer.
      FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, and DESIGN.md
    -->
    <Topbar {pathAggregate} />
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
        <div class="workspace">
          <div class="peer-pane" hidden={$view === 'settings'}>
            <PeerList {pathAggregate} />
          </div>
          <div class="chat-pane" hidden={$view === 'settings' || !$chatOpen}>
            <ChatView />
          </div>
          <div class="settings-pane" hidden={$view !== 'settings'}>
            <SettingsView />
          </div>
        </div>
      </main>
    </div>
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
    background: var(--surface);
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
    transition: background var(--dur-mid, 280ms) var(--ease-out, ease);
  }
  .drag-handle:hover, .drag-handle.active, .drag-handle:focus-visible {
    background: var(--border-hover);
  }
  .main-area {
    flex: 1;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    background: var(--bg);
    min-width: 0;
    min-height: 0;
  }
  .workspace {
    flex: 1;
    display: flex;
    min-height: 0;
    min-width: 0;
  }
  .peer-pane, .settings-pane {
    flex: 1;
    min-width: 0;
    min-height: 0;
    display: flex;
    flex-direction: column;
  }
  .peer-pane[hidden], .chat-pane[hidden], .settings-pane[hidden] {
    display: none !important;
  }
  .chat-pane {
    flex: 0 0 42%;
    max-width: 480px;
    min-width: 280px;
    min-height: 0;
    display: flex;
    flex-direction: column;
    border-left: 1px solid var(--border);
  }
  .error-strip {
    position: fixed;
    left: 16px;
    right: 16px;
    bottom: 16px;
    z-index: 1100;
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 12px 14px;
    background: var(--surface-2, var(--bg-raised, #111));
    border: 1px solid var(--fault, var(--error, #c41e1e));
    border-radius: var(--radius, 2px);
    color: var(--fault, var(--error, #c41e1e));
    font-size: var(--font-size-sm, 13px);
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
    border: 1px solid var(--fault, var(--error, #c41e1e));
    border-radius: var(--radius, 2px);
    background: transparent;
    color: var(--fault, var(--error, #c41e1e));
    cursor: pointer;
    font-family: inherit;
    font-size: 13px;
    text-transform: none;
  }
  .error-strip-dismiss:hover {
    background: color-mix(in srgb, var(--fault, #c41e1e) 12%, transparent);
  }
  .notification {
    position: fixed;
    top: 16px;
    right: 16px;
    padding: 12px 16px;
    font-size: 13px;
    font-weight: 500;
    z-index: 1000;
    background: var(--surface-2, var(--bg-raised, #111));
    border: 1px solid var(--border, #2a2a2a);
    border-radius: var(--radius, 2px);
    color: var(--text, var(--text-bright, #f5f5f5));
    max-width: min(360px, calc(100vw - 2rem));
  }
  .notification.error {
    border-color: var(--fault, var(--error, #c41e1e));
    color: var(--fault, var(--error, #c41e1e));
  }
</style>
