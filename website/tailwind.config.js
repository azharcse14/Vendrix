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
    // 9.6: choto lekha 14px/1.6, body 16px/1.7 (Bangla kar-fola).
    fontSize: { sm: ['0.875rem', { lineHeight: '1.6' }], base: ['1rem', { lineHeight: '1.7' }] },
    borderRadius: { DEFAULT: 'var(--radius)' },
    // Font file ashar age ba na thakle system font (Bangla shoho).
    fontFamily: { heading: ['var(--font-heading)', 'sans-serif'], body: ['var(--font-body)', 'sans-serif'] },
  }},
};
