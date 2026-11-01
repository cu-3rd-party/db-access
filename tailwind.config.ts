import type {Config} from 'tailwindcss';

// Палитра темы cu3rd из Forgejo (homelab apps/forgejo/values.yaml и forgejo-dark, на котором она основана).
// Цвета сайта меняются здесь, шаблоны используют только эти имена.
export default {
  content: ['./templates/**/*.tmpl', './web_src/js/**/*.ts'],
  theme: {
    extend: {
      colors: {
        page: '#050505',
        line: '#1c1c1c',
        code: '#0a0a0a',
        ink: {DEFAULT: '#dddddd', light: '#b1b1b1'},
        primary: {DEFAULT: '#dddddd', hover: '#c4c4c4', contrast: '#050505'},
        secondary: {DEFAULT: '#2e2e2e', hover: '#3a3a3a'},
        warning: {bg: '#644821', border: '#a67a1d', text: '#fff388'},
        danger: {bg: '#5f2525', border: '#783030', text: '#fef2f2'},
      },
      fontFamily: {
        sans: ['-apple-system', '"Segoe UI"', 'system-ui', '"Noto Sans"', 'Roboto', '"Helvetica Neue"', 'Arial', '"Liberation Sans"', 'sans-serif'],
        mono: ['ui-monospace', 'SFMono-Regular', '"SF Mono"', 'Menlo', 'Monaco', 'Consolas', '"Liberation Mono"', '"Courier New"', 'monospace'],
      },
      borderRadius: {box: '4px'},
    },
  },
  plugins: [],
} satisfies Config;
