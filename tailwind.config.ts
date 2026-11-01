import type {Config} from 'tailwindcss';

export default {
  content: ['./templates/**/*.tmpl', './web_src/js/**/*.ts'],
  darkMode: 'media',
  theme: {extend: {}},
  plugins: [],
} satisfies Config;
