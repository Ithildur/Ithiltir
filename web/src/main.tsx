import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';

import '@fontsource/space-grotesk/400.css';
import '@fontsource/space-grotesk/500.css';
import '@fontsource/space-grotesk/600.css';

import App from './App';
import './index.css';
import { TopBannerProvider } from '@components/ui/TopBannerStack';
import { AuthProvider } from '@context/AuthContext';
import { SiteBrandProvider } from '@context/SiteBrandContext';
import { ThemeProvider } from '@context/ThemeContext';
import { I18nProvider } from '@i18n';
import { TrafficRebuildProvider } from './providers/TrafficRebuildProvider';

const root = document.getElementById('root');

if (!root) {
  throw new Error('Root element not found');
}

const renderApp = () => {
  createRoot(root).render(
    <StrictMode>
      <I18nProvider>
        <TopBannerProvider>
          <SiteBrandProvider>
            <AuthProvider>
              <TrafficRebuildProvider>
                <ThemeProvider>
                  <App />
                </ThemeProvider>
              </TrafficRebuildProvider>
            </AuthProvider>
          </SiteBrandProvider>
        </TopBannerProvider>
      </I18nProvider>
    </StrictMode>,
  );
};

renderApp();
