import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import path from 'path';

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@cogs/brand': path.resolve(__dirname, '../../../docs/brand/react'),
    },
  },
  build: {
    outDir: path.resolve(__dirname, '../internal/ui/dist'),
    emptyOutDir: true,
  },
  base: '/',
});
