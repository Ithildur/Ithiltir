import React from 'react';
import type { DashUpdateChannel, DashUpdateMode, SystemSettings } from '@app-types/admin';
import { DashUpdateView } from './DashUpdateView';
import { useDashUpdate } from './hooks/useDashUpdate';

interface Props {
  enabled: boolean;
  settings: SystemSettings | null;
  loadingSettings: boolean;
  savingChannel: boolean;
  savingMode: boolean;
  onChannelChange: (channel: DashUpdateChannel) => void;
  onModeChange: (mode: DashUpdateMode) => void;
}

const DashUpdateSettings: React.FC<Props> = ({
  enabled,
  settings,
  loadingSettings,
  savingChannel,
  savingMode,
  onChannelChange,
  onModeChange,
}) => {
  const controller = useDashUpdate({
    enabled,
    channel: settings?.dash_update_channel ?? 'release',
    onChannelChange,
  });

  return (
    <DashUpdateView
      settings={settings}
      loadingSettings={loadingSettings}
      savingChannel={savingChannel}
      savingMode={savingMode}
      onModeChange={onModeChange}
      controller={controller}
    />
  );
};

export default DashUpdateSettings;
