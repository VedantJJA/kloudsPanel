<script lang="ts">
  import { Check, Eye, EyeOff } from 'lucide-svelte';
  let email = $state('');
  let displayName = $state('');
  let password = $state('');
  let password2 = $state('');
  let showPassword = $state(false);
  let showPassword2 = $state(false);
  let error = $state('');
  let success = $state(false);
  let loading = $state(false);

  function handlePasswordInput() {
    if (error === 'Passwords do not match' && password === password2) {
      error = '';
    } else if (error && error !== 'Passwords do not match') {
      error = '';
    }
  }

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
        error = data.detail ?? data.error ?? 'Signup failed';
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
  <title>Request Access - kloudsPanel</title>
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
        Create an account on kloudsPanel
      </p>
    </div>

    {#if success}
      <div style="
        background:#d1fae5;border:1px solid #6ee7b7;color:#065f46;
        border-radius:var(--radius-md);padding:1rem 1.25rem;
        font-size:0.875rem;text-align:center;
      ">
        <strong style="display:flex;align-items:center;justify-content:center;gap:0.5rem;"><Check size={16} /> Request submitted!</strong><br/>
        Your account is ready. You can now sign in.
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
          <label class="form-label" for="name">Name</label>
          <input 
            id="name" 
            type="text" 
            class="form-input" 
            bind:value={displayName}
            placeholder="Your name" 
            required 
            autocomplete="name"
          />
        </div>

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
              oninput={handlePasswordInput}
              placeholder="Minimum 8 characters" 
              required 
              minlength="8"
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
                <EyeOff size={18} />
              {:else}
                <Eye size={18} />
              {/if}
            </button>
          </div>
        </div>

        <div class="form-group">
          <label class="form-label" for="password2">Confirm Password</label>
          <div style="position: relative; display: flex; align-items: center;">
            <input 
              id="password2" 
              type={showPassword2 ? 'text' : 'password'} 
              class="form-input" 
              bind:value={password2}
              oninput={handlePasswordInput}
              placeholder="Re-enter password" 
              required 
              minlength="8"
              style="padding-right: 2.5rem;"
            />
            <button
              type="button"
              onclick={() => showPassword2 = !showPassword2}
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
              title={showPassword2 ? 'Hide password' : 'Show password'}
              aria-label={showPassword2 ? 'Hide password' : 'Show password'}
            >
              {#if showPassword2}
                <EyeOff size={18} />
              {:else}
                <Eye size={18} />
              {/if}
            </button>
          </div>
        </div>

        <button type="submit" class="btn btn-primary w-full" disabled={loading} style="margin-top: 0.5rem;">
          {#if loading}Submitting...{:else}Request Access{/if}
        </button>
      </form>

      <p style="text-align:center;font-size:0.875rem;color:var(--color-ink-secondary);margin-top:1.5rem">
        Already have an account? <a href="/login" style="color:var(--color-accent);font-weight:500">Sign in</a>
      </p>
    {/if}
  </div>
</div>
