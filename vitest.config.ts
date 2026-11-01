import {defineConfig} from 'vitest/config';

export default defineConfig({
  test: {
    include: ['web_src/**/*.test.ts'],
    environment: 'happy-dom',
    globals: true,
    testTimeout: 20000,
  },
});
