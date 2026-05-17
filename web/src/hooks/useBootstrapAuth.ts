import React from 'react';
import { useAuth } from '@context/AuthContext';

export const useBootstrapAuth = (): void => {
  const { bootstrap, status } = useAuth();

  React.useEffect(() => {
    if (status !== 'unknown') return;
    void bootstrap();
  }, [bootstrap, status]);
};
