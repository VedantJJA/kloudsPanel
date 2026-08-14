<script lang="ts">
  import { goto } from '$app/navigation';
  let email = $state('');
  let password = $state('');
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
        error = data.detail ?? 'Invalid credentials';
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
  background: var(--color-rail);
  padding: 1rem;
">
  <div style="
    background: var(--color-surface);
    border-radius: var(--radius-lg);
    padding: 2.5rem;
    width: 100%;
    max-width: 420px;
    box-shadow: 0 24px 64px rgba(0,0,0,0.25);
  ">
    <!-- Logo -->
    <div style="text-align:center;margin-bottom:2rem">
      <div style="
        width:52px;height:52px;background:var(--color-accent);
        border-radius:var(--radius-md);display:inline-flex;
        align-items:center;justify-content:center;
        font-size:1.5rem;font-weight:700;color:#fff;margin-bottom:1rem
      ">K</div>
      <h1 style="font-size:1.375rem;font-weight:700;color:var(--color-ink);margin:0">kloudsPanel</h1>
      <p style="font-size:0.875rem;color:var(--color-ink-secondary);margin-top:0.25rem">Sign in to your account</p>
    </div>

    {#if error}
      <div style="
        background:#fee2e2;border:1px solid #fca5a5;color:#991b1b;
        border-radius:var(--radius-md);padding:0.75rem 1rem;
        font-size:0.875rem;margin-bottom:1.25rem;
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
          placeholder="name"
          required
          autocomplete="email"
        />
      </div>

      <div class="form-group">
        <label class="form-label" for="password">Password</label>
        <input
          id="password"
          type="password"
          class="form-input"
          bind:value={password}
          placeholder="••••••••"
          required
          autocomplete="current-password"
        />
      </div>

      <button type="submit" class="btn btn-primary w-full" disabled={loading}>
        {#if loading}Signing in…{:else}Sign In{/if}
      </button>
    </form>

    <p style="text-align:center;font-size:0.875rem;color:var(--color-ink-secondary);margin-top:1.5rem">
      Don't have an account?
      <a href="/signup" style="color:var(--color-accent);font-weight:500">Request access</a>
    </p>
  </div>
</div>
