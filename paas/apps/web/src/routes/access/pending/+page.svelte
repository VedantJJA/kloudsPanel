<script lang="ts">
  import { Clock, ArrowLeft } from 'lucide-svelte';
</script>

<svelte:head>
  <title>Pending Approval - kloudsPanel</title>
</svelte:head>

<div style="
  min-height:100vh;display:flex;align-items:center;justify-content:center;
  background:var(--color-canvas);padding:1rem;
">
  <div class="card" style="
    background:var(--color-surface);border-radius:var(--radius-lg);
    border:1px solid var(--color-border);
    padding:2.5rem 2rem;width:100%;max-width:440px;
    text-align:center;
  ">
    <div style="margin-bottom:1.25rem;display:flex;justify-content:center;color:var(--color-ink-muted);">
      <Clock size={48} />
    </div>
    <h1 style="font-size:1.25rem;margin-bottom:0.5rem;color:var(--color-ink);font-weight:700;">Pending Approval</h1>
    <p style="color:var(--color-ink-secondary);font-size:0.875rem;line-height:1.6">
      Your account has been created and is pending approval from the platform administrator.
      You will receive access once your account is approved.
    </p>
    <div style="
      background:var(--color-surface-subtle);border:1px solid var(--color-border);border-radius:var(--radius-md);
      padding:0.75rem 1rem;margin-top:1.25rem;font-size:0.8125rem;color:var(--color-ink-secondary);
    ">
      Contact your administrator if you need urgent access.
    </div>
    <a href="/login" class="btn btn-secondary" style="
      display:inline-flex;align-items:center;gap:6px;margin-top:1.5rem;
      font-weight:500;font-size:0.8125rem;
    ">
      <ArrowLeft size={14} /> Back to Sign In
    </a>
  </div>
</div>
