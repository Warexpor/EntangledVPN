<script>
  import { onMount, onDestroy } from 'svelte'
  import { status, view, addNotification, peers, clearDurableError } from '../stores/app.js'
  import { t, fmt } from '../locales/index.js'

  let savedRooms = []
  let showCreate = false
  let showJoin = false
  let roomName = ''
  let roomPass = ''
  let joinName = ''
  let joinPass = ''
  let createError = ''
  let joinError = ''
  let createNameErr = ''
  let joinNameErr = ''
  let serverAddr = ''
  let nickname = ''
  let joining = ''
  let menuOpen = null
  let createDialog
  let joinDialog

  onMount(async () => {
    savedRooms = await window.go.main.App.GetSavedRooms()
    const cfg = await window.go.main.App.LoadConfig()
    if (cfg.serverAddr) serverAddr = cfg.serverAddr
    if (cfg.nickname) nickname = cfg.nickname
    window.addEventListener('keydown', onGlobalKey)
    window.addEventListener('click', onGlobalClick)
  })

  onDestroy(() => {
    window.removeEventListener('keydown', onGlobalKey)
    window.removeEventListener('click', onGlobalClick)
  })

  function onGlobalKey(e) {
    if (e.key === 'Escape') {
      if (menuOpen) { menuOpen = null; return }
      if (showCreate) { closeCreate(); return }
      if (showJoin) { closeJoin(); return }
    }
  }

  function onGlobalClick() {
    menuOpen = null
  }

  async function loadSavedRooms() {
    savedRooms = await window.go.main.App.GetSavedRooms()
  }

  function openCreate() {
    createError = ''
    createNameErr = ''
    showCreate = true
    clearDurableError()
  }

  function closeCreate() {
    showCreate = false
    createError = ''
    createNameErr = ''
  }

  function openJoin() {
    joinError = ''
    joinNameErr = ''
    showJoin = true
    clearDurableError()
  }

  function closeJoin() {
    showJoin = false
    joinError = ''
    joinNameErr = ''
  }

  async function createRoom() {
    createNameErr = ''
    createError = ''
    if (!roomName.trim()) {
      createNameErr = $t.field_required
      return
    }
    try {
      await window.go.main.App.CreateRoom(roomName, roomPass)
      addNotification(fmt($t.notif_created, { name: roomName }))
      $view = 'network'
      closeCreate()
      roomName = ''
      roomPass = ''
      await loadSavedRooms()
    } catch (e) {
      createError = fmt($t.error_create, { err: e })
      addNotification(createError, 'error')
    }
  }

  async function joinRoom(name, password) {
    joinNameErr = ''
    joinError = ''
    const n = name || joinName
    const p = password !== undefined ? password : joinPass
    if (!n.trim()) {
      joinNameErr = $t.field_required
      return
    }
    try {
      await window.go.main.App.JoinRoom(n, p)
      addNotification(fmt($t.notif_joined, { name: n }))
      $view = 'network'
      closeJoin()
      joinName = ''
      joinPass = ''
      await loadSavedRooms()
    } catch (e) {
      joinError = fmt($t.error_join, { err: e })
      addNotification(joinError, 'error')
      if (!showJoin) {
        joinName = n
        joinPass = ''
        showJoin = true
      }
    }
  }

  async function leaveRoom() {
    if (!confirm($t.confirm_leave)) return
    try {
      await window.go.main.App.LeaveRoom()
      addNotification($t.notif_left)
      await loadSavedRooms()
    } catch (e) {
      addNotification(fmt($t.error_leave, { err: e }), 'error')
    }
  }

  async function removeSaved(name) {
    const active = $status.room === name
    if (active) {
      if (!confirm($t.confirm_leave_remove)) return
    } else if (!confirm(fmt($t.confirm_remove, { name }))) {
      return
    }
    try {
      if (active) await window.go.main.App.LeaveRoom()
      await window.go.main.App.RemoveSavedRoom(name)
      addNotification(fmt($t.notif_removed, { name }))
      await loadSavedRooms()
    } catch (e) {
      addNotification(fmt($t.error_remove, { err: e }), 'error')
    }
  }

  async function deleteNetwork(name) {
    if (!confirm(fmt($t.confirm_delete, { name }))) return
    try {
      await window.go.main.App.DeleteRoom(name)
      await window.go.main.App.RemoveSavedRoom(name)
      addNotification(fmt($t.notif_deleted, { name }))
      await loadSavedRooms()
    } catch (e) {
      addNotification(String(e), 'error')
    }
  }

  async function copyInvite(room) {
    try {
      const invite = await window.go.main.App.FormatInvite(room.name, room.password || '')
      await window.go.main.App.CopyText(invite)
      const parts = String(invite).split('|')
      const hasPass = parts.length >= 3 && parts[2] !== ''
      if (room.locked && !hasPass) {
        addNotification($t.invite_no_password, 'info')
      } else {
        addNotification($t.invite_copied)
      }
    } catch (e) {
      addNotification(String(e), 'error')
    }
  }

  async function refreshConfig() {
    try {
      const cfg = await window.go.main.App.LoadConfig()
      if (cfg.serverAddr) serverAddr = cfg.serverAddr
      if (cfg.nickname) nickname = cfg.nickname
    } catch (_) {}
  }

  async function switchRoom(name, password, server) {
    if (joining) return
    joining = name
    menuOpen = null
    try {
      await refreshConfig()
      const addr = server || serverAddr
      if (!$status.connected) {
        if (!addr || !nickname) {
          addNotification($t.notif_need_config, 'error')
          $view = 'connect'
          return
        }
        const s = await window.go.main.App.Connect(addr, nickname)
        status.set(s)
        await loadSavedRooms()
      }
      await window.go.main.App.JoinRoom(name, password || '')
      $view = 'network'
      await loadSavedRooms()
    } catch (e) {
      const msg = fmt($t.error_join, { err: e })
      addNotification(msg, 'error')
      joinName = name
      joinPass = ''
      joinError = msg
      showJoin = true
    } finally {
      joining = ''
    }
  }

  function onRowKey(e, room) {
    if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault()
      switchRoom(room.name, room.password, room.server)
    }
  }

  function toggleMenu(e, name) {
    e.stopPropagation()
    menuOpen = menuOpen === name ? null : name
  }

  function menuAction(e, fn) {
    e.stopPropagation()
    menuOpen = null
    fn()
  }
</script>

<aside class="sidebar" aria-label={$t.networks}>
  <div class="sidebar-header">
    <h2>{$t.networks}</h2>
    <div class="sidebar-meta">
      {fmt($t.network_count, { n: savedRooms.length })}
    </div>
  </div>

  <div class="sidebar-actions">
    <button class="btn" on:click={openCreate} disabled={!$status.connected}>
      {$t.create_network}
    </button>
    <button class="btn" on:click={openJoin} disabled={!$status.connected}>
      {$t.join_network}
    </button>
  </div>

  <div class="network-list" role="list">
    {#each savedRooms as room (room.name)}
      <div
        class="network-item"
        class:active={$status.room === room.name}
        class:disabled={!$status.connected && !joining}
        role="button"
        tabindex="0"
        aria-current={$status.room === room.name ? 'true' : undefined}
        aria-label={room.name}
        on:click={() => switchRoom(room.name, room.password, room.server)}
        on:keydown={(e) => onRowKey(e, room)}
      >
        <div class="network-indicator" class:active-dot={$status.room === room.name} aria-hidden="true"></div>
        <div class="network-info">
          <div class="network-name">{room.name}</div>
          <div class="network-meta">
            {#if joining === room.name}
              {$t.connecting}
            {:else if $status.room === room.name}
              {fmt($t.peer_count, { n: $status.peerCount })}
            {:else if !$status.connected}
              {$t.offline_click}
            {:else}
              {$t.saved}
            {/if}
          </div>
        </div>
        <div class="row-actions">
          <button
            type="button"
            class="menu-btn"
            aria-haspopup="menu"
            aria-expanded={menuOpen === room.name}
            aria-label={$t.more_actions}
            title={$t.more_actions}
            on:click={(e) => toggleMenu(e, room.name)}
          ><svg width="16" height="16" viewBox="0 0 16 16" aria-hidden="true">
              <circle cx="8" cy="3.5" r="1.25" fill="currentColor" />
              <circle cx="8" cy="8" r="1.25" fill="currentColor" />
              <circle cx="8" cy="12.5" r="1.25" fill="currentColor" />
            </svg></button>
          {#if menuOpen === room.name}
            <div class="overflow-menu" role="menu" tabindex="-1" on:click|stopPropagation on:keydown|stopPropagation>
              <button type="button" role="menuitem" class="menu-item" on:click={(e) => menuAction(e, () => copyInvite(room))}>
                {$t.copy_invite}
              </button>
              {#if $status.room === room.name}
                <button type="button" role="menuitem" class="menu-item" on:click={(e) => menuAction(e, leaveRoom)}>
                  {$t.leave}
                </button>
              {/if}
              <button type="button" role="menuitem" class="menu-item danger" on:click={(e) => menuAction(e, () => removeSaved(room.name))}>
                {$t.remove}
              </button>
              {#if $status.room === room.name && $status.isOwner}
                <button type="button" role="menuitem" class="menu-item danger" on:click={(e) => menuAction(e, () => deleteNetwork(room.name))}>
                  {$t.delete_room}
                </button>
              {/if}
            </div>
          {/if}
        </div>
      </div>
    {:else}
      <div class="empty-state">
        <div>{$t.no_networks}</div>
        <div class="empty-desc">{$t.no_networks_desc}</div>
      </div>
    {/each}
  </div>

  {#if showCreate}
    <div
      class="modal-overlay"
      role="presentation"
      on:click={closeCreate}
      on:keydown={(e) => e.key === 'Escape' && closeCreate()}
    >
      <!-- svelte-ignore a11y-no-noninteractive-element-interactions -->
      <div
        class="modal"
        role="dialog"
        aria-modal="true"
        aria-labelledby="create-title"
        bind:this={createDialog}
        on:click|stopPropagation
        on:keydown|stopPropagation
      >
        <h3 id="create-title">{$t.create_network}</h3>
        <label class="sr-only" for="create-name">{$t.network_name}</label>
        <input
          id="create-name"
          type="text"
          placeholder={$t.network_name}
          bind:value={roomName}
          class:invalid={createNameErr}
          aria-invalid={!!createNameErr}
        />
        {#if createNameErr}
          <span class="field-error" role="alert">{createNameErr}</span>
        {/if}
        <label class="sr-only" for="create-pass">{$t.password_opt}</label>
        <input id="create-pass" type="password" placeholder={$t.password_opt} bind:value={roomPass} />
        {#if createError}
          <div class="modal-error" role="alert">{createError}</div>
        {/if}
        <div class="modal-actions">
          <button class="btn" on:click={closeCreate}>{$t.cancel}</button>
          <button class="btn primary" on:click={createRoom}>{$t.create}</button>
        </div>
      </div>
    </div>
  {/if}

  {#if showJoin}
    <div
      class="modal-overlay"
      role="presentation"
      on:click={closeJoin}
      on:keydown={(e) => e.key === 'Escape' && closeJoin()}
    >
      <!-- svelte-ignore a11y-no-noninteractive-element-interactions -->
      <div
        class="modal"
        role="dialog"
        aria-modal="true"
        aria-labelledby="join-title"
        bind:this={joinDialog}
        on:click|stopPropagation
        on:keydown|stopPropagation
      >
        <h3 id="join-title">{$t.join_network}</h3>
        <label class="sr-only" for="join-name">{$t.network_name}</label>
        <input
          id="join-name"
          type="text"
          placeholder={$t.network_name}
          bind:value={joinName}
          class:invalid={joinNameErr}
          aria-invalid={!!joinNameErr}
        />
        {#if joinNameErr}
          <span class="field-error" role="alert">{joinNameErr}</span>
        {/if}
        <label class="sr-only" for="join-pass">{$t.password_opt}</label>
        <input id="join-pass" type="password" placeholder={$t.password_opt} bind:value={joinPass} />
        {#if joinError}
          <div class="modal-error" role="alert">{joinError}</div>
        {/if}
        <div class="modal-actions">
          <button class="btn" on:click={closeJoin}>{$t.cancel}</button>
          <button class="btn primary" on:click={() => joinRoom()}>{$t.join}</button>
        </div>
      </div>
    </div>
  {/if}
</aside>

<style>
  .sidebar {
    width: 100%;
    height: 100%;
    background: var(--bg-surface);
    display: flex;
    flex-direction: column;
    flex-shrink: 0;
  }
  .sidebar-header {
    padding: 12px;
    border-bottom: 1px solid var(--border);
  }
  .sidebar-header h2 {
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--text-dim);
    font-weight: 600;
  }
  .sidebar-meta {
    font-size: var(--font-size-xs);
    color: var(--text-muted);
    margin-top: 4px;
  }
  .sidebar-actions {
    padding: 8px;
    display: flex;
    flex-direction: column;
    gap: 6px;
    border-bottom: 1px solid var(--border);
  }
  .sidebar-actions .btn {
    min-height: 32px;
    padding: 6px 12px;
  }
  .network-list {
    flex: 1;
    overflow-y: auto;
    overflow-x: hidden;
    padding: 4px 0;
  }
  .network-item {
    display: flex;
    align-items: center;
    padding: 10px 12px;
    min-height: 40px;
    cursor: pointer;
    gap: 8px;
    border-bottom: 1px solid var(--border-deep);
    transition: background 0.12s ease, opacity 0.12s ease;
    position: relative;
  }
  .network-item:hover, .network-item:focus-visible {
    background: var(--bg-hover);
  }
  .network-item.active {
    background: var(--bg-active);
  }
  .network-item.disabled {
    opacity: 0.55;
  }
  .network-indicator {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--text-dim);
    flex-shrink: 0;
  }
  .network-indicator.active-dot {
    background: var(--text-bright);
  }
  .network-info {
    flex: 1;
    min-width: 0;
  }
  .network-name {
    font-size: var(--font-size);
    color: var(--text-bright);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .network-meta {
    font-size: var(--font-size-xs);
    color: var(--text-secondary);
    line-height: 1.4;
  }
  .row-actions {
    position: relative;
    flex-shrink: 0;
  }
  .menu-btn {
    background: none;
    border: 1px solid transparent;
    border-radius: var(--radius-sm, 4px);
    color: var(--text-muted);
    cursor: pointer;
    width: 32px;
    height: 32px;
    display: flex;
    align-items: center;
    justify-content: center;
  }
  .menu-btn:hover, .menu-btn[aria-expanded="true"] {
    color: var(--text-bright);
    border-color: var(--border);
    background: var(--bg-raised);
  }
  .overflow-menu {
    position: absolute;
    right: 0;
    top: 100%;
    z-index: 20;
    min-width: 160px;
    background: var(--bg-raised);
    border: 1px solid var(--border);
    border-radius: var(--radius-md, 6px);
    display: flex;
    flex-direction: column;
    box-shadow: 0 4px 12px rgba(0,0,0,0.35);
  }
  .menu-item {
    background: none;
    border: none;
    text-align: left;
    padding: 10px 12px;
    min-height: 36px;
    color: var(--text-primary);
    font-family: inherit;
    font-size: var(--font-size-sm);
    cursor: pointer;
  }
  .menu-item:hover { background: var(--bg-hover); color: var(--text-bright); }
  .menu-item.danger { color: var(--fault, var(--error)); }
  .menu-item.danger:hover { background: rgba(226, 61, 61, 0.1); }
  .empty-desc {
    margin-top: 6px;
    font-size: var(--font-size-xs);
    color: var(--text-dim);
  }
  .empty-state {
    padding: 16px;
    text-align: center;
    color: var(--text-muted);
    font-size: var(--font-size-sm);
  }
  .modal-overlay {
    position: fixed;
    inset: 0;
    background: rgba(0,0,0,0.7);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 100;
    animation: fadeIn 0.12s ease-out;
  }
  @keyframes fadeIn {
    from { opacity: 0; }
    to { opacity: 1; }
  }
  .modal {
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: var(--radius-md, 6px);
    padding: 20px;
    width: 320px;
    max-height: 80vh;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: 10px;
  }
  .modal h3 {
    font-size: var(--font-size);
    color: var(--text-bright);
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    font-size: 10px;
  }
  .modal input {
    padding: 10px 10px;
    min-height: 36px;
    border: 1px solid var(--border);
    border-radius: var(--radius-sm, 4px);
    background: var(--bg-primary);
    color: var(--text-primary);
    font-size: var(--font-size);
    font-family: var(--font-mono);
  }
  .modal input.invalid {
    border-color: var(--error);
  }
  .field-error, .modal-error {
    font-size: var(--font-size-xs);
    color: var(--error);
  }
  .modal-error {
    padding: 8px;
    border: 1px solid var(--error);
    background: rgba(196, 92, 92, 0.08);
  }
  .modal-actions {
    display: flex;
    gap: 6px;
    justify-content: flex-end;
  }
  .modal-actions .btn { min-height: 32px; padding: 6px 12px; }
  .sr-only {
    position: absolute;
    width: 1px;
    height: 1px;
    padding: 0;
    margin: -1px;
    overflow: hidden;
    clip: rect(0, 0, 0, 0);
    border: 0;
  }
</style>
