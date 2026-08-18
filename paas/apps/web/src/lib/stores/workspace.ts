import { writable, derived } from 'svelte/store';

export interface Workspace {
  id: string;
  ID?: string;
  name: string;
  Name?: string;
  slug: string;
  Slug?: string;
  role?: string;
  createdAt?: string;
  updatedAt?: string;
}

export const workspaces = writable<Workspace[]>([]);
export const activeWorkspace = writable<Workspace | null>(null);
export const isWorkspaceLoading = writable<boolean>(false);

export const activeWorkspaceSlug = derived(activeWorkspace, ($ws) => $ws?.slug || $ws?.Slug || '');
export const activeWorkspaceId = derived(activeWorkspace, ($ws) => $ws?.id || $ws?.ID || '');

const STORAGE_KEY = 'klouds_active_workspace_slug';

export function setActiveWorkspace(ws: Workspace | null) {
  activeWorkspace.set(ws);
  if (typeof window !== 'undefined' && ws) {
    const slug = ws.slug || ws.Slug || '';
    if (slug) {
      try {
        localStorage.setItem(STORAGE_KEY, slug);
      } catch {}
    }
  }
}

export async function loadWorkspaces(preferredSlug?: string): Promise<Workspace[]> {
  isWorkspaceLoading.set(true);
  try {
    const res = await fetch('/api/v1/workspaces', { credentials: 'include' });
    if (!res.ok) {
      isWorkspaceLoading.set(false);
      return [];
    }
    const data = await res.json();
    const list: Workspace[] = data.workspaces || [];
    workspaces.set(list);

    let savedSlug = '';
    if (typeof window !== 'undefined') {
      try {
        savedSlug = localStorage.getItem(STORAGE_KEY) || '';
      } catch {}
    }

    const targetSlug = preferredSlug || savedSlug;
    let selected: Workspace | null = null;

    if (targetSlug && list.length > 0) {
      selected = list.find(w => (w.slug || w.Slug) === targetSlug || (w.id || w.ID) === targetSlug) || null;
    }

    if (!selected && list.length > 0) {
      selected = list[0];
    }

    if (selected) {
      setActiveWorkspace(selected);
    }

    isWorkspaceLoading.set(false);
    return list;
  } catch (err) {
    console.error('Failed to load workspaces:', err);
    isWorkspaceLoading.set(false);
    return [];
  }
}

export async function createNewWorkspace(name: string): Promise<Workspace | null> {
  try {
    const res = await fetch('/api/v1/workspaces', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'include',
      body: JSON.stringify({ name })
    });
    if (!res.ok) {
      return null;
    }
    const newWs = await res.json();
    const list = await loadWorkspaces(newWs.slug || newWs.Slug);
    const created = list.find(w => (w.slug || w.Slug) === (newWs.slug || newWs.Slug)) || newWs;
    setActiveWorkspace(created);
    return created;
  } catch (err) {
    console.error('Failed to create workspace:', err);
    return null;
  }
}
