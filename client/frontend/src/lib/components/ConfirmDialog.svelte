<script>
  import { createEventDispatcher, tick } from 'svelte'
  import { t } from '../locales/index.js'

  export let open = false
  export let title = ''
  export let message = ''
  export let confirmLabel = ''
  export let danger = false

  const dispatch = createEventDispatcher()
  let dialog
  let cancelButton

  $: if (open) {
    tick().then(() => cancelButton?.focus())
  }

  function close() {
    dispatch('cancel')
  }

  function confirm() {
    dispatch('confirm')
  }

  function onKeydown(event) {
    if (event.key === 'Escape') {
      event.preventDefault()
      close()
      return
    }
    if (event.key !== 'Tab' || !dialog) return
    const focusable = [...dialog.querySelectorAll('button:not(:disabled)')]
    if (!focusable.length) return
    const first = focusable[0]
    const last = focusable[focusable.length - 1]
    if (event.shiftKey && document.activeElement === first) {
      event.preventDefault()
      last.focus()
    } else if (!event.shiftKey && document.activeElement === last) {
      event.preventDefault()
      first.focus()
    }
  }
</script>

{#if open}
  <div class="confirm-overlay" role="presentation" on:click|self={close} on:keydown={onKeydown}>
    <div class="confirm-dialog" bind:this={dialog} role="alertdialog" aria-modal="true" aria-labelledby="confirm-title" aria-describedby="confirm-message">
      <h2 id="confirm-title">{title || $t.confirm_title}</h2>
      <p id="confirm-message">{message}</p>
      <div class="confirm-actions">
        <button type="button" class="btn ghost" bind:this={cancelButton} on:click={close}>{$t.cancel}</button>
        <button type="button" class:danger class="btn primary" on:click={confirm}>{confirmLabel || $t.confirm}</button>
      </div>
    </div>
  </div>
{/if}

<style>
  .confirm-overlay {
    position: fixed;
    inset: 0;
    z-index: var(--z-modal);
    display: grid;
    place-items: center;
    padding: 20px;
    background: rgba(0, 0, 0, 0.72);
    animation: confirmFade 0.14s ease-out;
  }
  .confirm-dialog {
    width: min(380px, 100%);
    padding: 20px;
    background: var(--bg-surface);
    border: 1px solid var(--border-light);
    box-shadow: 0 12px 32px rgba(0, 0, 0, 0.34);
  }
  h2 {
    margin: 0 0 10px;
    color: var(--text-bright);
    font-size: calc(var(--font-size) * 1.2);
    font-weight: 600;
  }
  p {
    margin: 0;
    color: var(--text-secondary);
    font-size: var(--font-size-sm);
    line-height: 1.55;
  }
  .confirm-actions {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    margin-top: 20px;
  }
  .confirm-actions .danger {
    background: var(--error);
    border-color: var(--error);
    color: #fff;
  }
  @keyframes confirmFade {
    from { opacity: 0; }
    to { opacity: 1; }
  }
  @media (prefers-reduced-motion: reduce) {
    .confirm-overlay { animation: none; }
  }
</style>
