/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{js,ts,jsx,tsx}'],
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        // Orange primary
        primary: {
          50:  '#fff7ed',
          100: '#ffedd5',
          200: '#fed7aa',
          300: '#fdba74',
          400: '#fb923c',
          500: '#f97316',
          600: '#ea580c',
          700: '#c2410c',
          800: '#9a3412',
          900: '#7c2d12',
        },
        // Dark theme surfaces
        surface: {
          950: '#07070f',
          900: '#0b0b16',
          850: '#0e0e1d',
          800: '#121222',
          750: '#161628',
          700: '#1c1c30',
          600: '#22223a',
          500: '#2c2c48',
          400: '#383860',
        },
      },
      backgroundImage: {
        'gradient-blue':       'linear-gradient(90deg, #3b82f6 0%, #1d4ed8 100%)',
        'gradient-blue-soft':  'linear-gradient(90deg, #60a5fa 0%, #3b82f6 100%)',
        'gradient-orange':     'linear-gradient(135deg, #fb923c 0%, #ea580c 100%)',
        'gradient-primary':    'linear-gradient(135deg, #f97316 0%, #c2410c 100%)',
        'gradient-sidebar':    'linear-gradient(180deg, #0e0e1d 0%, #0b0b16 100%)',
        'gradient-card':       'linear-gradient(135deg, rgba(249,115,22,0.06) 0%, rgba(59,130,246,0.04) 100%)',
        'gradient-hero':       'linear-gradient(135deg, #f97316 0%, #3b82f6 100%)',
      },
      fontFamily: {
        sans: ['Inter', 'system-ui', '-apple-system', 'sans-serif'],
      },
      animation: {
        'fade-in':      'fadeIn 0.25s ease-out',
        'slide-in-left':'slideInLeft 0.3s ease-out',
        'slide-up':     'slideUp 0.25s ease-out',
        'scale-in':     'scaleIn 0.2s ease-out',
        'pulse-slow':   'pulse 3s ease-in-out infinite',
        'shimmer':      'shimmer 2s infinite',
      },
      keyframes: {
        fadeIn:       { from: { opacity: '0', transform: 'translateY(6px)' }, to: { opacity: '1', transform: 'translateY(0)' } },
        slideInLeft:  { from: { opacity: '0', transform: 'translateX(-12px)' }, to: { opacity: '1', transform: 'translateX(0)' } },
        slideUp:      { from: { opacity: '0', transform: 'translateY(10px)' }, to: { opacity: '1', transform: 'translateY(0)' } },
        scaleIn:      { from: { opacity: '0', transform: 'scale(0.96)' }, to: { opacity: '1', transform: 'scale(1)' } },
        shimmer:      { '0%': { backgroundPosition: '-200% 0' }, '100%': { backgroundPosition: '200% 0' } },
      },
      boxShadow: {
        'glow-orange': '0 0 24px rgba(249, 115, 22, 0.25)',
        'glow-blue':   '0 0 24px rgba(59, 130, 246, 0.25)',
        'card':        '0 1px 3px rgba(0,0,0,0.4), 0 1px 2px rgba(0,0,0,0.24)',
        'card-hover':  '0 8px 24px rgba(0,0,0,0.5), 0 2px 8px rgba(0,0,0,0.3)',
        'sidebar':     '1px 0 0 rgba(255,255,255,0.05)',
      },
      borderRadius: {
        'xl':  '12px',
        '2xl': '16px',
        '3xl': '24px',
      },
      transitionDuration: {
        '200': '200ms',
        '300': '300ms',
      },
    },
  },
  plugins: [],
}
