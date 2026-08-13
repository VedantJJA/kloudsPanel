<script lang="ts">
  import { goto } from '$app/navigation';
  let email = $state('');
  let displayName = $state('');
  let password = $state('');
  let password2 = $state('');
  let error = $state('');
  let success = $state(false);
  let loading = $state(false);

  async function handleSignup(e: Event) {
    e.preventDefault();
    if (password !== password2) {
      error = 'Passwords do not match';
      return;
    }
    loading = true;
    error = '';
    try {
      const res = await fetch('/api/v1/auth/signup', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email, display_name: displayName, password }),
      });
      if (!res.ok) {
        const data = await res.json();
        error = data.detail ?? 'Signup failed';
        return;
      }
      success = true;
    } catch {
      error = 'Network error. Please try again.';
    } finally {
      loading = false;
    }
  }
</script>

<svelte:head>
  <title>Request Access — kloudsPanel</title>
</svelte:head>

<div style="
  min-height:100vh;display:flex;align-items:center;justify-content:center;
  background:var(--color-rail);padding:1rem;
">
  <div style="
    background:var(--color-surface);border-radius:var(--radius-lg);
    padding:2.5rem;width:100%;max-width:440px;
    box-shadow:0 24px 64px rgba(0,0,0,0.25);
  ">
    <div style="text-align:center;margin-bottom:2rem">
      <div style="
        width:52px;height:52px;background:var(--color-accent);
        border-radius:var(--radius-md);display:inline-flex;
        align-items:center;justify-content:center;
        font-size:1.5rem;font-weight:700;color:#fff;margin-bottom:1rem
      ">K</div>
      <h1 style="font-size:1.375rem;font-weight:700;color:var(--color-ink);margin:0">Request Access</h1>
      <p style="font-size:0.875rem;color:var(--color-ink-secondary);margin-top:0.25rem">
        New accounts require admin approval
      </p>
    </div>

    {#if success}
      <div style="
        background:#d1fae5;border:1px solid #6ee7b7;color:#065f46;
        border-radius:var(--radius-md);padding:1rem 1.25rem;
        font-size:0.875rem;text-align:center;
      ">
        <strong>✓ Request submitted!</strong><br/>
        Your account is pending approval. You'll be notified when access is granted.
        <br/><br/>
        <a href="/login" style="color:var(--color-accent-dim);font-weight:500">Back to Sign In →</a>
      </div>
    {:else}
      {#if error}
        <div style="
          background:#fee2e2;border:1px solid #fca5a5;color:#991b1b;
          border-radius:var(--radius-md);padding:0.75rem 1rem;
          font-size:0.875rem;margin-bottom:1.25rem;
        ">{error}</div>
      {/if}

      <form onsubmit={handleSignup}>
        <div class="form-group">
          <label class="form-label" for="name">Full Name</label>
          <input id="name" type="text" class="form-input" bind:value={displayName}
            placeholder="Jane Smith" required autocomplete="name"/>
        </div>
        <div class="form-group">
          <label class="form-label" for="email">Email Address</label>
          <input id="email" type="email" class="form-input" bind:value={email}
            placeholder="you@example.com" required autocomplete="email"/>
        </div>
        <div class="form-group">
          <label class="form-label" for="password">Password</label>
          <input id="password" type="password" class="form-input" bind:value={password}
            placeholder="Minimum 8 characters" required minlength="8"/>
        </div>
        <div class="form-group">
          <label class="form-label" for="password2">Confirm Password</label>
          <input id="password2" type="password" class="form-input" bind:value={password2}
            placeholder="••••••••" required minlength="8"/>
        </div>
        <button type="submit" class="btn btn-primary w-full" disabled={loading}>
          {#if loading}Submitting…{:else}Request Access{/if}
        </button>
      </form>

      <p style="text-align:center;font-size:0.875rem;color:var(--color-ink-secondary);margin-top:1.5rem">
        Already have an account? <a href="/login" style="color:var(--color-accent);font-weight:500">Sign in</a>
      </p>
    {/if}
  </div>
</div>
