import js from '@eslint/js';
import betterTailwindcss from 'eslint-plugin-better-tailwindcss';
import reactHooks from 'eslint-plugin-react-hooks';
import tseslint from 'typescript-eslint';

export default tseslint.config(
  {
    ignores: ['dist/**', 'node_modules/**', '.prettierrc.cjs'],
  },
  js.configs.recommended,
  ...tseslint.configs.recommended,
  {
    plugins: {
      'better-tailwindcss': betterTailwindcss,
      'react-hooks': reactHooks,
    },
    settings: {
      'better-tailwindcss': {
        entryPoint: './src/index.css',
        rootFontSize: 16,
      },
    },
    rules: {
      ...reactHooks.configs.recommended.rules,
      'better-tailwindcss/enforce-canonical-classes': 'error',
      'better-tailwindcss/no-unnecessary-whitespace': 'error',
      'react-hooks/exhaustive-deps': 'warn',
      'react-hooks/immutability': 'off',
    },
  },
);
