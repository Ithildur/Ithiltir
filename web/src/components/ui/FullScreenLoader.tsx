import React from 'react';
import { useI18n } from '@i18n';

const FullScreenLoader: React.FC = () => {
  const { t } = useI18n();

  return (
    <div
      className="min-h-screen bg-(--theme-bg-muted) dark:bg-(--theme-bg-default) flex items-center justify-center"
      role="status"
      aria-live="polite"
    >
      <div
        className="size-6 border-2 border-(--theme-border-hover) dark:border-(--theme-border-default) border-t-(--theme-fg-interactive) rounded-full animate-spin"
        aria-hidden="true"
      />
      <span className="sr-only">{t('loading')}</span>
    </div>
  );
};

export default FullScreenLoader;
