// 9.3: rong, radius ar font shudhu CSS variable (`<head>` er `:root`, settings theke); hex na.
/** @type {import('tailwindcss').Config} */
module.exports = {
  content: ['./lib/**/*.dart'],
  theme: { extend: {
    colors: {
      primary: 'var(--color-primary)', 'primary-hover': 'var(--color-primary-hover)',
      'on-primary': 'var(--color-on-primary)', accent: 'var(--color-accent)',
      'on-accent': 'var(--color-on-accent)', tint: 'var(--color-tint)',
      ink: 'var(--color-text)', muted: 'var(--color-text-muted)',
      page: 'var(--color-bg)', surface: 'var(--color-surface)', line: 'var(--color-border)',
    },
    borderRadius: { DEFAULT: 'var(--radius)' },
    fontFamily: { heading: 'var(--font-heading)', body: 'var(--font-body)' },
  }},
};
