import React from 'react';

const FullScreenLoader: React.FC = () => (
  <div
    className="min-h-screen bg-(--theme-bg-muted) dark:bg-(--theme-bg-default) flex items-center justify-center"
    role="status"
    aria-live="polite"
  >
    <div className="size-6 border-2 border-(--theme-border-hover) dark:border-(--theme-border-default) border-t-(--theme-fg-interactive) rounded-full animate-spin" />
  </div>
);

export default FullScreenLoader;
