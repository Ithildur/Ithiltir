import { defineConfig, loadEnv } from 'vite';
import react from '@vitejs/plugin-react';
import tailwindcss from '@tailwindcss/vite';

const fromHere = (relativePath: string): string => {
  const pathname = decodeURIComponent(new URL(relativePath, import.meta.url).pathname);
  return /^\/[A-Za-z]:/.test(pathname) ? pathname.slice(1) : pathname;
};

const resolveDevApiTarget = (env: Record<string, string>): string => {
  return (
    env.front_test_api ??
    env.FRONT_TEST_API ??
    env.VITE_FRONT_TEST_API ??
    process.env.front_test_api ??
    process.env.FRONT_TEST_API ??
    process.env.VITE_FRONT_TEST_API ??
    ''
  ).trim();
};

export default defineConfig(({ command }) => {
  const root = fromHere('.');
  const testOnlyEnv = command === 'serve' ? loadEnv('test', root, '') : {};
  const devApiTarget = command === 'serve' ? resolveDevApiTarget(testOnlyEnv) : '';

  return {
    plugins: [react(), tailwindcss()],
    resolve: {
      alias: {
        '@components': fromHere('./src/components'),
        '@pages': fromHere('./src/pages'),
        '@lib': fromHere('./src/lib'),
        '@hooks': fromHere('./src/hooks'),
        '@context': fromHere('./src/context'),
        '@app-types': fromHere('./src/types'),
        '@utils': fromHere('./src/utils'),
        '@theme': fromHere('./src/theme'),
        '@config': fromHere('./src/config'),
        '@i18n': fromHere('./src/i18n'),
      },
    },
    server: {
      host: true,
      port: 5173,
      ...(devApiTarget
        ? {
            proxy: {
              '/api': {
                target: devApiTarget,
                changeOrigin: true,
                cookieDomainRewrite: '',
                ws: false,
              },
              '/theme': {
                target: devApiTarget,
                changeOrigin: true,
                ws: false,
              },
            },
          }
        : {}),
    },
  };
});
