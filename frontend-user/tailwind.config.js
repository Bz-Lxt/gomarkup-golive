/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{vue,ts}'],
  theme: {
    extend: {
      colors: {
        bg: '#070b10',
        panel: '#0e1620',
        panel2: '#121c28',
        line: '#1c2c3c',
        cyan: '#3ee0c5',
        amber: '#f0b429',
        danger: '#ff4d6d',
        phosphor: '#8cff6b',
        violet: '#9b8cff',
        ink: '#d7e4ef',
        mute: '#7b8b99',
      },
      fontFamily: {
        display: ['Oxanium', 'sans-serif'],
        mono: ['"Share Tech Mono"', 'ui-monospace', 'monospace'],
      },
      boxShadow: {
        glow: '0 0 24px rgba(62, 224, 197, 0.18)',
      },
    },
  },
  plugins: [],
}
