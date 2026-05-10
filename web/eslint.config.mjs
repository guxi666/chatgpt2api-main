import js from '@eslint/js';
import reactHooks from 'eslint-plugin-react-hooks';
import reactRefresh from 'eslint-plugin-react-refresh';
import globals from 'globals';
import prettier from 'eslint-config-prettier/flat';
import tseslint from 'typescript-eslint';

const eslintConfig = [
    { ignores: ['**/.next/**', '**/dist/**', '**/node_modules/**', '**/out/**'] },
    js.configs.recommended,
    ...tseslint.configs.recommended,
    {
        files: ['**/*.{ts,tsx}'],
        languageOptions: {
            ecmaVersion: 2020,
            globals: globals.browser,
        },
        plugins: {
            'react-hooks': reactHooks,
            'react-refresh': reactRefresh,
        },
        rules: {
            'react-hooks/rules-of-hooks': 'error',
            'react-hooks/exhaustive-deps': 'warn',
            'react-refresh/only-export-components': [
                'warn',
                { allowConstantExport: true, allowExportNames: ['badgeVariants', 'buttonVariants'] },
            ],
        },
    },
    prettier,
    {
        rules: {
            '@typescript-eslint/no-unused-vars': 'off', // 不检查未使用的变量
            '@typescript-eslint/no-explicit-any': 'off', // 关闭 any 报错
        },
    },
];

export default eslintConfig;
