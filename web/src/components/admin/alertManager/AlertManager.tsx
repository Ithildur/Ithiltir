import React from 'react';
import Bell from 'lucide-react/dist/esm/icons/bell';
import BellRing from 'lucide-react/dist/esm/icons/bell-ring';
import Plus from 'lucide-react/dist/esm/icons/plus';
import SlidersHorizontal from 'lucide-react/dist/esm/icons/sliders-horizontal';
import { useI18n } from '@i18n';
import { AdminSectionTabs } from '@components/admin/AdminSectionTabs';
import Button from '@components/ui/Button';
import Card from '@components/ui/Card';
import ConfirmDialog from '@components/ui/ConfirmDialog';
import AlertRuleTable from '@components/admin/alertManager/AlertRuleTable';
import AlertRuleModal from '@components/admin/alertManager/AlertRuleModal';
import AlertChannelsPanel from '@components/admin/alertManager/AlertChannelsPanel';
import AlertChannelModal from '@components/admin/alertManager/AlertChannelModal';
import AlertMountsPanel from '@components/admin/alertManager/AlertMountsPanel';
import { useAlertChannels } from '@components/admin/alertManager/hooks/useAlertChannels';
import { useAlertMounts } from '@components/admin/alertManager/hooks/useAlertMounts';
import { useAlertRules } from '@components/admin/alertManager/hooks/useAlertRules';
import { useConfirmDialog } from '@hooks/useConfirmDialog';

const tabs = [
  { key: 'config', labelKey: 'admin_alerts_tab_config', icon: Bell },
  { key: 'rules', labelKey: 'admin_alerts_tab_rules', icon: SlidersHorizontal },
  { key: 'channels', labelKey: 'admin_alerts_tab_channels', icon: BellRing },
] as const;

type AlertManagerTab = (typeof tabs)[number]['key'];

const AlertManager: React.FC = () => {
  const { t } = useI18n();
  const [activeTab, setActiveTab] = React.useState<AlertManagerTab>('config');
  const {
    dialogProps: confirmDialogProps,
    request: requestConfirm,
    run: confirmAction,
  } = useConfirmDialog();
  const rules = useAlertRules({
    enabled: activeTab === 'rules',
    confirm: requestConfirm,
    confirmAction,
  });
  const mounts = useAlertMounts({ enabled: activeTab === 'config' });
  const channels = useAlertChannels({
    enabled: activeTab === 'channels',
    confirmAction,
  });

  return (
    <div className="space-y-4 md:space-y-6">
      <ConfirmDialog {...confirmDialogProps} />

      <div className="flex flex-col md:flex-row justify-between gap-3 md:gap-4">
        <div className="flex w-full md:w-auto md:flex-1">
          <AdminSectionTabs tabs={tabs} activeKey={activeTab} onChange={setActiveTab} />
        </div>

        {activeTab === 'rules' && (
          <div className="flex w-full md:w-auto gap-2 items-center">
            <Button
              icon={Plus}
              className="w-full md:w-auto shadow-(color:--theme-shadow-interactive)"
              onClick={rules.openAdd}
              disabled={rules.isModalOpen}
            >
              {t('admin_alerts_add_rule')}
            </Button>
          </div>
        )}
        {activeTab === 'channels' && (
          <div className="flex w-full md:w-auto gap-2 items-center">
            <Button
              icon={Plus}
              className="w-full md:w-auto shadow-(color:--theme-shadow-interactive)"
              onClick={channels.openAdd}
            >
              {t('admin_alerts_channels_add')}
            </Button>
          </div>
        )}
      </div>

      {activeTab === 'config' ? (
        <AlertMountsPanel
          rules={mounts.rules}
          nodes={mounts.nodes}
          loading={mounts.loading}
          saving={mounts.saving}
          onSetMounts={mounts.setMounts}
        />
      ) : activeTab === 'rules' ? (
        <Card className="overflow-hidden">
          <AlertRuleTable
            rules={rules.rules}
            loading={rules.loading}
            togglingIds={rules.togglingIds}
            onToggleEnabled={rules.toggleEnabled}
            renamingIds={rules.renamingIds}
            onRename={rules.rename}
            onEdit={rules.openEdit}
            onDelete={rules.deleteRule}
          />
        </Card>
      ) : (
        <AlertChannelsPanel
          channels={channels.channels}
          loading={channels.loading}
          togglingIds={channels.togglingIds}
          testingIds={channels.testingIds}
          onToggleEnabled={channels.toggleEnabled}
          onEdit={channels.openEdit}
          onDelete={channels.deleteChannel}
          onTest={channels.testChannel}
        />
      )}

      {rules.isModalOpen && (
        <AlertRuleModal
          isOpen={rules.isModalOpen}
          initialRule={rules.editingRule}
          saving={rules.saving}
          onClose={rules.closeModal}
          onSave={rules.saveRule}
          onSuccess={rules.afterSave}
        />
      )}

      <AlertChannelModal
        isOpen={channels.isModalOpen}
        mode={channels.editingChannelId !== null ? 'edit' : 'add'}
        channelId={channels.editingChannelId ?? undefined}
        initialForm={channels.modalForm}
        isSaving={channels.saving}
        onClose={channels.closeModal}
        onSave={channels.saveChannel}
      />
    </div>
  );
};

export default AlertManager;
