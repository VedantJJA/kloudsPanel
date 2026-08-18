<script lang="ts">
  import { theme } from '$lib/stores/theme';

  interface Props {
    size?: number;
    width?: number;
    height?: number;
    variant?: 'auto' | 'light' | 'dark';
    showText?: boolean;
    class?: string;
  }

  let {
    size = 28,
    width,
    height,
    variant = 'auto',
    showText = false,
    class: className = ''
  }: Props = $props();

  const computedWidth = $derived(width ?? size * 1.714);
  const computedHeight = $derived(height ?? size);

  // In 'auto' mode, light theme gets dark cloud, dark theme gets white cloud
  const isDarkCloud = $derived(
    variant === 'light' || (variant === 'auto' && $theme === 'light')
  );
  const cloudFill = $derived(isDarkCloud ? '#090A0F' : '#FFFFFF');
  const koStroke = $derived(isDarkCloud ? 'none' : '#090A0F');
</script>

<div class="logo-wrapper {className}" style="display: inline-flex; align-items: center; gap: 8px;">
  <svg 
    width={computedWidth} 
    height={computedHeight} 
    viewBox="0 0 240 140" 
    fill="none" 
    xmlns="http://www.w3.org/2000/svg"
    style="flex-shrink: 0; display: block;"
  >
    <!-- Cloud Silhouette -->
    <path 
      d="M 68 115 C 38 115 18 96 18 70 C 18 46 36 28 59 27 C 69 11 90 0 114 0 C 144 0 168 18 174 42 C 196 43 214 60 214 81 C 214 100 198 115 178 115 Z" 
      fill={cloudFill} 
    />
    <!-- K.O Neon Lime Glyphs (#C6F806) -->
    <g fill="#C6F806">
      <!-- Letter K -->
      <path 
        d="M 40 44 C 40 40 43 37 47 37 L 85 37 C 88 37 90 40 88 43 L 68 62 L 91 97 C 93 100 90 104 86 104 L 50 104 C 44 104 40 100 40 94 Z" 
        stroke={koStroke} 
        stroke-width={isDarkCloud ? '0' : '1.5'} 
      />
      <!-- Dot . -->
      <circle 
        cx="98" 
        cy="94" 
        r="7" 
        stroke={koStroke} 
        stroke-width={isDarkCloud ? '0' : '1.5'} 
      />
      <!-- Letter O -->
      <path 
        fill-rule="evenodd" 
        clip-rule="evenodd" 
        d="M 115 50 C 115 42 122 36 130 36 L 165 36 C 173 36 180 42 180 50 L 180 90 C 180 98 173 104 165 104 L 130 104 C 122 104 115 98 115 90 Z M 135 56 C 133 56 132 57 132 59 L 132 81 C 132 83 133 84 135 84 L 160 84 C 162 84 163 83 163 81 L 163 59 C 163 57 162 56 160 56 Z" 
        stroke={koStroke} 
        stroke-width={isDarkCloud ? '0' : '1.5'} 
      />
    </g>
  </svg>

  {#if showText}
    <span style="font-weight: 700; font-size: 1.05rem; letter-spacing: -0.02em; color: var(--color-ink);">
      kloudsPanel
    </span>
  {/if}
</div>
