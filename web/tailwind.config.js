/** @type {import('tailwindcss').Config} */
export default {
  darkMode: 'class',
  content: ['./index.html', './src/**/*.{ts,tsx}'],
  theme: {
    extend: {
      colors: {
        surface: {
          DEFAULT: '#f8f9fa',
          dark: '#0f1117',
        },
        panel: {
          DEFAULT: '#ffffff',
          dark: '#1a1d27',
        },
        border: {
          DEFAULT: '#e2e5ea',
          dark: '#2a2d3a',
        },
        accent: {
          DEFAULT: '#00e5ff',
          dim: '#00b8d4',
          glow: 'rgba(0, 229, 255, 0.15)',
        },
        warn: {
          DEFAULT: '#ffab00',
          dim: '#ff8f00',
        },
        muted: {
          DEFAULT: '#6b7280',
          dark: '#8b95a5',
        },
      },
      fontFamily: {
        sans: ['"DM Sans"', 'system-ui', 'sans-serif'],
        mono: ['"JetBrains Mono"', 'ui-monospace', 'monospace'],
      },
    },
  },
  plugins: [],
}
