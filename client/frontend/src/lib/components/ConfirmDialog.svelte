<script>
  import { t } from '../locales/index.js'

  export let open = false
  export let message = ''
  export let confirmLabel = ''
  export let danger = false

  export let onConfirm = () => {}
  export let onCancel = () => {}

  function cancel() {
    onCancel()
  }

  function confirm() {
    onConfirm()
  }

  function onOverlayKey(e) {
    if (e.key === 'Escape') cancel()
  }
</script>

{#if open}
  <div
    class="modal-overlay"
    role="presentation"
    on:click={cancel}
    on:keydown={onOverlayKey}
  >
    <!-- svelte-ignore a11y-no-noninteractive-element-interactions -->
    <div
      class="modal"
      role="dialog"
      aria-modal="true"
      aria-labelledby="confirm-msg"
      on:click|stopPropagation
      on:keydown|stopPropagation
    >
      <p id="confirm-msg">{message}</p>
      <div class="modal-actions">
        <button type="button" class="btn" on:click={cancel}>{$t.cancel}</button>
        <button type="button" class="btn" class:danger class:btn-cta={!danger} on:click={confirm}>
          {confirmLabel || $t.disconnect}
        </button>
      </div>
    </div>
  </div>
{/if}

<style>
  .modal-overlay {
    position: fixed;
    inset: 0;
    background: color-mix(in srgb, var(--bg, #0a0a0a) 55%, transparent);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 200;
  }
  .modal {
    background: var(--surface, #141414);
    border: 1px solid var(--border, #2a2a2a);
    border-radius: var(--radius, 2px);
    padding: 20px;
    width: 340px;
    display: flex;
    flex-direction: column;
    gap: 16px;
  }
  .modal p {
    color: var(--text, #f5f5f5);
    font-size: 14px;
    line-height: 1.45;
  }
  .modal-actions {
    display: flex;
    gap: 8px;
    justify-content: flex-end;
  }
</style>
