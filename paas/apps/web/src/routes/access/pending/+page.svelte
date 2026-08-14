<script lang="ts">
  import { Clock } from 'lucide-svelte';
</script>

<svelte:head>
  <title>Pending Approval - kloudsPanel</title>
</svelte:head>

<div style="
  min-height:100vh;display:flex;align-items:center;justify-content:center;
  background:var(--color-rail);padding:1rem;
">
  <div class="auth-card" style="
    background:var(--color-surface);border-radius:var(--radius-lg);
    padding:3rem 2.5rem;width:100%;max-width:480px;
    box-shadow:0 24px 64px rgba(0,0,0,0.25);text-align:center;
  ">
    <div class="text-muted" style="margin-bottom:1.5rem;display:flex;justify-content:center;">
      <Clock size={64} />
    </div>
    <h1 style="font-size:1.5rem;margin-bottom:0.5rem;color:var(--color-ink)">Pending Approval</h1>
    <p style="color:var(--color-ink-secondary);font-size:0.9375rem;line-height:1.7">
      Your account has been created and is pending approval from the platform administrator.
      You'll receive access once your account is approved.
    </p>
    <div style="
      background:var(--color-canvas);border-radius:var(--radius-md);
      padding:1rem;margin-top:1.5rem;font-size:0.875rem;color:var(--color-ink-secondary);
    ">
      Contact your administrator if you need urgent access.
    </div>
    <a href="/login" style="
      display:inline-block;margin-top:1.5rem;color:var(--color-accent);
      font-weight:500;font-size:0.875rem;
    ">← Back to Sign In</a>
  </div>
</div>
