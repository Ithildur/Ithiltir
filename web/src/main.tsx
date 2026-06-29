import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';

import '@fontsource/space-grotesk/400.css';
import '@fontsource/space-grotesk/500.css';
import '@fontsource/space-grotesk/600.css';

import App from './App';
import './index.css';
import { I18nRuntime } from '@runtime/I18nRuntime';
import { AuthRuntime } from '@runtime/AuthRuntime';
import { SiteBrandRuntime } from '@runtime/SiteBrandRuntime';
import { ThemeRuntime } from '@runtime/ThemeRuntime';
import { TopBannerHost } from '@runtime/TopBannerHost';
import { SearchShortcutRuntime } from '@runtime/SearchShortcutRuntime';
import { installBrowserRuntime } from '@runtime/bootstrap';

const root = document.getElementById('root');

if (!root) {
  throw new Error('Root element not found');
}

installBrowserRuntime();

const renderApp = () => {
  createRoot(root).render(
    <StrictMode>
      <I18nRuntime />
      <TopBannerHost />
      <SiteBrandRuntime />
      <AuthRuntime />
      <ThemeRuntime />
      <SearchShortcutRuntime />
      <App />
    </StrictMode>,
  );
};

renderApp();
