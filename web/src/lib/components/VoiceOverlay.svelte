<script lang="ts">
import { voice } from "$lib/stores/voice.svelte";
import { onMount, onDestroy } from "svelte";
import { Icon } from "svelte-icons-pack";
import { FiX, FiMic } from "svelte-icons-pack/fi";

onMount(() => {
  voice.start();
});

onDestroy(() => {
  voice.stop();
});

let stateLabel = $derived(
  voice.state === "connecting"
    ? "Connecting..."
    : voice.state === "listening"
      ? "Listening"
      : voice.state === "processing"
        ? "Thinking..."
        : voice.state === "speaking"
          ? "Speaking"
          : "",
);
</script>

<div class="voice-overlay">
  <button class="close-btn" onclick={() => voice.stop()} title="Close voice mode">
    <Icon src={FiX} size="22" />
  </button>

  <div class="voice-content">
    <div class="orb" class:listening={voice.state === "listening"} class:processing={voice.state === "processing"} class:speaking={voice.state === "speaking"}>
      <div class="orb-inner">
        <Icon src={FiMic} size="32" />
      </div>
      <div class="orb-ring ring-1"></div>
      <div class="orb-ring ring-2"></div>
      <div class="orb-ring ring-3"></div>
    </div>

    <p class="state-label">{stateLabel}</p>

    {#if voice.transcript}
      <p class="transcript">{voice.transcript}</p>
    {/if}

    {#if voice.response}
      <p class="response">{voice.response}</p>
    {/if}

    {#if voice.error}
      <p class="error-msg">{voice.error}</p>
    {/if}
  </div>
</div>

<style>
  .voice-overlay {
    position: fixed;
    inset: 0;
    z-index: 100;
    background: var(--color-bg);
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
  }

  .close-btn {
    position: absolute;
    top: max(1rem, env(safe-area-inset-top, 0px));
    right: 1rem;
    z-index: 10;
    display: flex;
    align-items: center;
    justify-content: center;
    width: 44px;
    height: 44px;
    border-radius: 50%;
    border: none;
    background: var(--color-overlay-medium);
    color: var(--color-text-secondary);
    cursor: pointer;
    transition: background 0.15s, color 0.15s;
  }
  .close-btn:hover {
    background: var(--color-overlay-strong);
    color: var(--color-text-primary);
  }

  .voice-content {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 1.5rem;
    padding: 2rem;
    max-width: 480px;
    width: 100%;
  }

  .orb {
    position: relative;
    width: 120px;
    height: 120px;
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .orb-inner {
    width: 80px;
    height: 80px;
    border-radius: 50%;
    background: var(--color-bg-tertiary);
    display: flex;
    align-items: center;
    justify-content: center;
    color: var(--color-text-muted);
    z-index: 1;
    transition: background 0.3s, color 0.3s;
  }

  .listening .orb-inner {
    background: var(--color-accent);
    color: white;
  }

  .processing .orb-inner {
    background: var(--color-surface);
    color: var(--color-text-secondary);
    animation: pulse-scale 1.5s ease-in-out infinite;
  }

  .speaking .orb-inner {
    background: var(--color-success);
    color: white;
  }

  .orb-ring {
    position: absolute;
    inset: 0;
    border-radius: 50%;
    border: 1.5px solid transparent;
    opacity: 0;
    transition: opacity 0.3s;
  }

  .listening .orb-ring {
    border-color: var(--color-accent);
    animation: ripple 2s ease-out infinite;
  }
  .listening .ring-2 { animation-delay: 0.4s; }
  .listening .ring-3 { animation-delay: 0.8s; }

  .speaking .orb-ring {
    border-color: var(--color-success);
    animation: ripple 1.5s ease-out infinite;
  }
  .speaking .ring-2 { animation-delay: 0.3s; }
  .speaking .ring-3 { animation-delay: 0.6s; }

  @keyframes ripple {
    0% {
      transform: scale(1);
      opacity: 0.5;
    }
    100% {
      transform: scale(1.8);
      opacity: 0;
    }
  }

  @keyframes pulse-scale {
    0%, 100% { transform: scale(1); }
    50% { transform: scale(1.05); }
  }

  .state-label {
    font-size: 14px;
    color: var(--color-text-muted);
    text-transform: uppercase;
    letter-spacing: 0.1em;
    margin: 0;
  }

  .transcript {
    font-size: 15px;
    color: var(--color-text-secondary);
    text-align: center;
    margin: 0;
    font-style: italic;
    line-height: 1.5;
  }

  .response {
    font-size: 16px;
    color: var(--color-text-primary);
    text-align: center;
    margin: 0;
    line-height: 1.5;
    max-height: 40vh;
    overflow-y: auto;
  }

  .error-msg {
    font-size: 13px;
    color: var(--color-error);
    margin: 0;
  }
</style>
