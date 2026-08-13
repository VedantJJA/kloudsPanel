<script lang="ts">
  import { Database } from 'lucide-svelte';
</script>

<div class="header">
  <div class="header-left">
    <h1>Databases</h1>
    <p class="text-muted">Manage your managed databases.</p>
  </div>
  <button class="btn btn-primary">
    <Database size={16} />
    New Database
  </button>
</div>

<div class="card">
  <div class="empty-state">
    <div class="empty-state-icon">
      <Database size={48} />
    </div>
    <p>No databases found in this workspace.</p>
  </div>
</div>
