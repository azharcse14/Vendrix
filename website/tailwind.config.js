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
      'primary-soft': 'var(--color-primary-soft)', 'accent-soft': 'var(--color-accent-soft)',
      success: 'var(--color-success)', 'success-soft': 'var(--color-success-soft)',
      danger: 'var(--color-danger)', 'danger-soft': 'var(--color-danger-soft)',
      warning: 'var(--color-warning)', 'warning-soft': 'var(--color-warning-soft)',
      focus: 'var(--color-focus)',
    },
    // 9.6: choto lekha 14px/1.6, body 16px/1.7 (Bangla kar-fola).
    fontSize: { sm: ['0.875rem', { lineHeight: '1.6' }], base: ['1rem', { lineHeight: '1.7' }] },
    borderRadius: { DEFAULT: 'var(--radius)', sm: 'var(--radius-sm)', lg: 'var(--radius-lg)' },
    // Font file ashar age ba na thakle system Bangla font (9.6 v1.38: swap e layout kom nore).
    fontFamily: {
      heading: ['var(--font-heading)', '"Noto Sans Bengali"', '"Nirmala UI"', 'sans-serif'],
      body: ['var(--font-body)', '"Noto Sans Bengali"', '"Nirmala UI"', 'sans-serif'],
    },
    // 9.3 v1.38: derived token, shudhu var().
    boxShadow: { 1: 'var(--shadow-1)', 2: 'var(--shadow-2)' },
  }},
};
