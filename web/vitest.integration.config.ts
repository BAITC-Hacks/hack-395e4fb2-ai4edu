import {defineConfig} from 'vitest/config';
import react from '@vitejs/plugin-react';
export default defineConfig({plugins:[react()],test:{environment:'jsdom',setupFiles:['./tests/setup.ts'],include:['tests/live-flow.tsx'],testTimeout:30000}});
