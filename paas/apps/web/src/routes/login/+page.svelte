<script lang="ts">
  import { goto } from '$app/navigation';
  import { Eye, EyeOff, Loader2 } from 'lucide-svelte';

  let email = $state('');
  let password = $state('');
  let showPassword = $state(false);
  let error = $state('');
  let loading = $state(false);

  async function handleLogin(e: Event) {
    e.preventDefault();
    loading = true;
    error = '';
    try {
      const res = await fetch('/api/v1/auth/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({ email, password }),
      });
      if (!res.ok) {
        const data = await res.json();
        error = data.detail ?? data.error ?? 'Invalid credentials';
        return;
      }
      goto('/workspaces');
    } catch (e) {
      error = 'Network error. Please try again.';
    } finally {
      loading = false;
    }
  }
</script>

<svelte:head>
  <title>Sign In - kloudsPanel</title>
</svelte:head>

<div style="
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--color-canvas);
  padding: 1rem;
">
  <div class="card" style="
    background: var(--color-surface);
    border-radius: var(--radius-lg);
    border: 1px solid var(--color-border);
    padding: 2.25rem 2rem;
    width: 100%;
    max-width: 400px;
  ">
    <!-- Logo -->
    <div style="text-align:center;margin-bottom:1.75rem">
      <div style="
        width:40px;height:40px;background:#ffffff;
        border-radius:var(--radius-sm);display:inline-flex;
        align-items:center;justify-content:center;
        font-size:1.25rem;font-weight:800;color:#090a0f;margin-bottom:0.75rem
      ">K</div>
      <h1 style="font-size:1.25rem;font-weight:700;color:var(--color-ink);margin:0">kloudsPanel</h1>
      <p style="font-size:0.8125rem;color:var(--color-ink-muted);margin-top:0.25rem">Sign in to your PaaS deployment console</p>
    </div>

    {#if error}
      <div style="
        background:var(--color-danger-subtle);border:1px solid rgba(248,113,113,0.3);color:var(--color-danger);
        border-radius:var(--radius-md);padding:0.65rem 0.85rem;
        font-size:0.8125rem;margin-bottom:1.25rem;
      ">{error}</div>
    {/if}

    <form onsubmit={handleLogin}>
      <div class="form-group">
        <label class="form-label" for="email">Email</label>
        <input
          id="email"
          type="email"
          class="form-input"
          bind:value={email}
          placeholder="name@example.com"
          required
          autocomplete="email"
        />
      </div>

      <div class="form-group">
        <label class="form-label" for="password">Password</label>
        <div style="position: relative; display: flex; align-items: center;">
          <input
            id="password"
            type={showPassword ? 'text' : 'password'}
            class="form-input"
            bind:value={password}
            placeholder="Enter password"
            required
            autocomplete="current-password"
            style="padding-right: 2.5rem;"
          />
          <button
            type="button"
            onclick={() => showPassword = !showPassword}
            style="
              position: absolute;
              right: 0.5rem;
              background: transparent;
              border: none;
              color: var(--color-ink-muted);
              cursor: pointer;
              display: flex;
              align-items: center;
              justify-content: center;
              padding: 0.25rem;
            "
            title={showPassword ? 'Hide password' : 'Show password'}
            aria-label={showPassword ? 'Hide password' : 'Show password'}
          >
            {#if showPassword}
              <EyeOff size={16} />
            {:else}
              <Eye size={16} />
            {/if}
          </button>
        </div>
      </div>

      <button type="submit" class="btn btn-primary w-full" disabled={loading} style="margin-top: 0.5rem; justify-content: center;">
        {#if loading}
          <Loader2 size={14} class="animate-spin" /> Signing in...
        {:else}
          Sign In
        {/if}
      </button>
    </form>

    <p style="text-align:center;font-size:0.8125rem;color:var(--color-ink-muted);margin-top:1.5rem">
      Don't have an account?
      <a href="/signup" style="color:var(--color-ink);font-weight:600;text-decoration:underline;">Request access</a>
    </p>
  </div>
</div>
