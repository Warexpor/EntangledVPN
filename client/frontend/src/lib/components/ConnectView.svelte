<script>
  import { onMount } from 'svelte'
  import { status, view, settings, addNotification, clearDurableError } from '../stores/app.js'
  import { t, fmt } from '../locales/index.js'
  import ConfirmDialog from './ConfirmDialog.svelte'

  let serverAddr = ''
  let nickname = ''
  let invite = ''
  let connecting = false
  let showInvite = false
  let fieldErrors = {}
  let showDisconnectConfirm = false

  onMount(async () => {
    const cfg = await window.go.main.App.LoadConfig()
    if (cfg.serverAddr) serverAddr = cfg.serverAddr
    if (cfg.nickname) nickname = cfg.nickname
    if (cfg) settings.set({ ...$settings, ...cfg })

    const manuallyDisconnected = localStorage.getItem('entangled_manual_disconnect') === '1'
    localStorage.removeItem('entangled_manual_disconnect')
    if (!manuallyDisconnected && cfg.autoConnect && cfg.serverAddr && cfg.nickname) {
      connect()
    }
  })

  function validateConnect() {
    const errs = {}
    if (!serverAddr.trim()) errs.serverAddr = $t.field_required
    if (!nickname.trim()) errs.nickname = $t.field_required
    fieldErrors = errs
    return Object.keys(errs).length === 0
  }

  async function connect() {
    if (connecting) return
    if (!validateConnect()) return
    connecting = true
    clearDurableError()
    try {
      const s = await window.go.main.App.Connect(serverAddr, nickname)
      status.set(s)
      $settings = { ...$settings, serverAddr, nickname }
      view.set('network')
      addNotification(fmt($t.notif_connected, { addr: serverAddr }))
      fieldErrors = {}
    } catch (e) {
      addNotification(fmt($t.error_connect, { err: e }), 'error')
    } finally {
      connecting = false
    }
  }

  async function applyInvite() {
    if (!invite.trim()) return
    clearDurableError()
    try {
      const p = await window.go.main.App.ParseInvite(invite.trim())
      if (!p.server || !p.room) {
        addNotification($t.error_invalid_invite, 'error')
        return
      }
      serverAddr = p.server
      if (!$status.connected) {
        if (!nickname.trim()) {
          fieldErrors = { ...fieldErrors, nickname: $t.field_required }
          addNotification($t.notif_need_config, 'error')
          return
        }
        connecting = true
        const s = await window.go.main.App.Connect(serverAddr, nickname)
        status.set(s)
        connecting = false
      }
      await window.go.main.App.JoinRoom(p.room, p.password || '')
      addNotification(fmt($t.notif_joined, { name: p.room }))
      view.set('network')
    } catch (e) {
      addNotification(fmt($t.error_join, { err: e }), 'error')
      connecting = false
    }
  }

  function handleNickKeydown(e) {
    if (e.key === 'Enter') {
      e.preventDefault()
      connect()
    }
  }

  function disconnect() {
    showDisconnectConfirm = true
  }

  async function executeDisconnect() {
    showDisconnectConfirm = false
    localStorage.setItem('entangled_manual_disconnect', '1')
    try {
      await window.go.main.App.Disconnect()
      status.set({ connected: false, reconnecting: false, server: '', room: '', virtualIP: '', peerCount: 0, isOwner: false, phase: 'idle' })
      view.set('connect')
      addNotification($t.notif_disconnected)
    } catch (e) {
      addNotification(fmt($t.error_disconnect, { err: e }), 'error')
    }
  }

  $: phaseLabel = $status.phase === 'dialing' ? $t.phase_dialing
    : $status.phase === 'auth' ? $t.phase_auth
    : $status.phase === 'ready' ? $t.phase_ready
    : $status.phase === 'error' ? $t.phase_error
    : ''
</script>

<div class="connect-view">
  {#if !$status.connected}
    <div class="connect-form">
      <div class="brand-block">
        <h1 class="brand">{$t.brand}</h1>
        <p class="tagline">{$t.tagline}</p>
      </div>

      <div class="field">
        <label for="server-input">{$t.server_addr}</label>
        <input
          id="server-input"
          type="text"
          bind:value={serverAddr}
          placeholder={$t.server_placeholder}
          class:invalid={fieldErrors.serverAddr}
          aria-invalid={!!fieldErrors.serverAddr}
          aria-describedby={fieldErrors.serverAddr ? 'server-err' : undefined}
          on:keydown={(e) => e.key === 'Enter' && (e.preventDefault(), connect())}
        />
        {#if fieldErrors.serverAddr}
          <span id="server-err" class="field-error" role="alert">{fieldErrors.serverAddr}</span>
        {/if}
      </div>
      <div class="field">
        <label for="nick-input">{$t.nickname}</label>
        <input
          id="nick-input"
          type="text"
          bind:value={nickname}
          placeholder={$t.nickname_placeholder}
          class:invalid={fieldErrors.nickname}
          aria-invalid={!!fieldErrors.nickname}
          aria-describedby={fieldErrors.nickname ? 'nick-err' : undefined}
          on:keydown={handleNickKeydown}
        />
        {#if fieldErrors.nickname}
          <span id="nick-err" class="field-error" role="alert">{fieldErrors.nickname}</span>
        {/if}
      </div>
      <button class="connect-btn" on:click={connect} disabled={connecting}>
        {connecting ? $t.connecting : $t.connect}
      </button>
      {#if connecting || $status.phase === 'dialing' || $status.phase === 'auth'}
        <div class="phase" role="status">{phaseLabel || $t.connecting}</div>
      {/if}

      <details class="invite-disclosure" bind:open={showInvite}>
        <summary>{$t.paste_invite}</summary>
        <div class="invite-block">
          <label for="invite-input" class="sr-only">{$t.paste_invite}</label>
          <input id="invite-input" type="text" bind:value={invite} placeholder={$t.invite_placeholder} on:keydown={(e) => e.key === 'Enter' && (e.preventDefault(), applyInvite())} />
          <button class="secondary-btn" on:click={applyInvite} disabled={connecting}>{$t.join}</button>
        </div>
      </details>
    </div>
  {:else}
    <div class="connected-info">
      {#if $status.reconnecting}
        <div class="connected-line" role="status">
          <span class="sq sq-warn" aria-hidden="true"></span>
          <span>{$t.reconnecting}</span>
        </div>
      {:else}
        <div class="connected-line" role="status">
          <span class="sq sq-green" aria-hidden="true"></span>
          <span>{$t.connected}</span>
        </div>
      {/if}
      <div class="info-line">
        <span>{$t.server}</span>
        <span class="info-value">{$status.server}</span>
      </div>
      <div class="info-line">
        <span>{$t.virtual_ip}</span>
        <span class="info-value">{$status.virtualIP || $t.na}</span>
      </div>
      <button class="disconnect-btn" on:click={disconnect}>{$t.disconnect}</button>
    </div>
  {/if}
</div>

<style>
  .connect-view {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 100%;
    height: 100%;
    padding: 32px 24px;
  }
  .connect-form, .connected-info {
    width: min(360px, 100%);
    display: flex;
    flex-direction: column;
    gap: 16px;
    animation: connectIn 0.45s cubic-bezier(0.16, 1, 0.3, 1) both;
  }
  @keyframes connectIn {
    from { opacity: 0; transform: translateY(12px); }
    to { opacity: 1; transform: translateY(0); }
  }
  .brand-block {
    text-align: center;
    margin-bottom: 12px;
  }
  .brand {
    font-family: var(--font-display);
    font-size: clamp(36px, 7vw, 48px);
    font-weight: 400;
    letter-spacing: 0.02em;
    text-transform: uppercase;
    color: var(--text-bright);
    line-height: 1.05;
  }
  .tagline {
    margin-top: 10px;
    font-size: var(--font-size-sm);
    color: var(--text-secondary);
    text-transform: none;
    letter-spacing: 0.02em;
    line-height: 1.45;
  }
  .field {
    display: flex;
    flex-direction: column;
    gap: 7px;
  }
  label {
    font-size: var(--font-size-xs);
    color: var(--text-secondary);
    text-transform: uppercase;
    letter-spacing: 0.08em;
  }
  input {
    background: var(--input-bg);
    border: 1px solid var(--border-light);
    color: var(--text-bright);
    padding: 13px 14px;
    min-height: 44px;
    font-family: var(--font-mono);
    font-size: var(--font-size);
    transition: border-color 0.15s ease, background 0.15s ease;
  }
  input::placeholder {
    color: var(--text-dim);
  }
  input:hover:not(:disabled) {
    border-color: var(--border-hover);
  }
  input:focus {
    border-color: var(--text-bright);
    background: var(--input-bg-focus);
  }
  input:focus-visible {
    outline: 2px solid var(--focus-ring);
    outline-offset: 2px;
  }
  input.invalid {
    border-color: var(--error);
  }
  .field-error {
    font-size: var(--font-size-xs);
    color: var(--error);
  }
  .connect-btn, .disconnect-btn, .secondary-btn {
    border: none;
    padding: 15px 18px;
    min-height: 48px;
    font-family: var(--font-mono);
    font-size: var(--font-size);
    cursor: pointer;
    text-transform: uppercase;
    letter-spacing: 0.12em;
    font-weight: 600;
    transition: background 0.15s ease, border-color 0.15s ease, transform 0.12s ease;
  }
  .connect-btn {
    margin-top: 4px;
    background: var(--accent);
    color: var(--accent-ink);
  }
  .connect-btn:hover:not(:disabled) {
    background: var(--accent-hover);
    transform: translateY(-1px);
  }
  .connect-btn:active:not(:disabled) {
    transform: translateY(0);
  }
  .connect-btn:disabled {
    opacity: 0.45;
    cursor: not-allowed;
    transform: none;
  }
  .secondary-btn {
    background: transparent;
    color: var(--text-primary);
    border: 1px solid var(--border-light);
    font-weight: 500;
    letter-spacing: 0.08em;
  }
  .secondary-btn:hover:not(:disabled) {
    border-color: var(--accent);
    color: var(--text-bright);
  }
  .disconnect-btn {
    background: transparent;
    color: var(--error);
    border: 1px solid var(--error);
    font-weight: 500;
  }
  .phase {
    font-size: var(--font-size-xs);
    color: var(--text-muted);
    text-align: center;
    letter-spacing: 0.04em;
  }
  .invite-disclosure {
    margin-top: 8px;
    border-top: 1px solid var(--border);
    padding-top: 14px;
  }
  .invite-disclosure summary {
    cursor: pointer;
    color: var(--text-muted);
    font-size: var(--font-size-sm);
    list-style: none;
    min-height: 36px;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: color 0.15s ease;
  }
  .invite-disclosure summary:hover {
    color: var(--text-secondary);
  }
  .invite-disclosure summary::-webkit-details-marker { display: none; }
  .invite-disclosure summary::before {
    content: '+';
    margin-right: 8px;
    color: var(--text-dim);
  }
  .invite-disclosure[open] summary::before { content: '−'; }
  .invite-block {
    display: flex;
    flex-direction: column;
    gap: 10px;
    margin-top: 12px;
  }
  .connected-line {
    display: flex;
    align-items: center;
    gap: 8px;
    color: var(--text-bright);
  }
  .info-line {
    display: flex;
    justify-content: space-between;
    font-size: var(--font-size-sm);
    color: var(--text-secondary);
  }
  .info-value {
    color: var(--text-primary);
    font-family: var(--font-mono);
  }
  .sq {
    width: 8px;
    height: 8px;
    display: inline-block;
  }
  .sq-green { background: var(--success); }
  .sq-warn { background: var(--warning); }
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

{#if showDisconnectConfirm}
  <ConfirmDialog
    open={true}
    message={$t.confirm_disconnect}
    danger={true}
    on:cancel={() => (showDisconnectConfirm = false)}
    on:confirm={executeDisconnect}
  />
{/if}
