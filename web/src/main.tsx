import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';

import App from './App';
import './index.css';
import { AuthRuntime } from '@runtime/AuthRuntime';
import { SiteBrandRuntime } from '@runtime/SiteBrandRuntime';
import { ThemeRuntime } from '@runtime/ThemeRuntime';
import { TopBannerHost } from '@runtime/TopBannerHost';
import { SearchShortcutRuntime } from '@runtime/SearchShortcutRuntime';
import { installAuthApiSession } from '@stores/authStore';
import { syncThemeRuntimeState } from '@stores/themeStore';

const root = document.getElementById('root');

if (!root) {
  throw new Error('Root element not found');
}

installAuthApiSession();
syncThemeRuntimeState();

createRoot(root).render(
  <StrictMode>
    <TopBannerHost />
    <SiteBrandRuntime />
    <AuthRuntime />
    <ThemeRuntime />
    <SearchShortcutRuntime />
    <App />
  </StrictMode>,
);
