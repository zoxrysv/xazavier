import {defineConfig} from 'vite';
import react from '@vitejs/plugin-react';

const API_TARGET = process.env.VITE_API_URL || 'http://localhost:8080';

// base './' lets the build run from any folder or subpath on a static host
export default defineConfig({
  plugins: [react()],
  base: './',
  server: {
    proxy: {
      '/api': {
        target: API_TARGET,
        changeOrigin: true,
        secure: false,
      },
    },
  },
});
