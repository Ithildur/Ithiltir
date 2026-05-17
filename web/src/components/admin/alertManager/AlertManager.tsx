import React from 'react';
import Bell from 'lucide-react/dist/esm/icons/bell';
import BellRing from 'lucide-react/dist/esm/icons/bell-ring';
import Plus from 'lucide-react/dist/esm/icons/plus';
import SlidersHorizontal from 'lucide-react/dist/esm/icons/sliders-horizontal';
import { useI18n } from '@i18n';
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

type SubTab = 'rules' | 'config' | 'channels';

const AlertManager: React.FC = () => {
  const { t } = useI18n();
  const [activeTab, setActiveTab] = React.useState<SubTab>('config');
  const confirmDialog = useConfirmDialog();
  const rules = useAlertRules({
    enabled: activeTab === 'rules',
    confirm: confirmDialog.request,
    confirmAction: confirmDialog.run,
  });
  const mounts = useAlertMounts({ enabled: activeTab === 'config' });
  const channels = useAlertChannels({
    enabled: activeTab === 'channels',
    confirmAction: confirmDialog.run,
  });

  return (
    <div className="space-y-4 md:space-y-6">
      <ConfirmDialog {...confirmDialog.dialogProps} />

      <div className="flex flex-col md:flex-row justify-between gap-3 md:gap-4">
        <div className="flex w-full md:w-auto md:flex-1">
          <div className="flex gap-4 border-b border-(--theme-border-subtle) dark:border-(--theme-border-default)">
            <button
              type="button"
              onClick={() => setActiveTab('config')}
              aria-current={activeTab === 'config' ? 'page' : undefined}
              className={`px-1 py-2 text-sm font-semibold border-b-2 -mb-px transition-colors ${
                activeTab === 'config'
                  ? 'border-(--theme-border-underline-nav-active) text-(--theme-fg-default)'
                  : 'border-transparent text-(--theme-fg-muted) dark:text-(--theme-fg-muted) hover:text-(--theme-fg-default) dark:hover:text-(--theme-fg-default)'
              }`}
            >
              <span className="inline-flex items-center gap-2">
                <Bell className="size-4" aria-hidden="true" />
                {t('admin_alerts_tab_config')}
              </span>
            </button>
            <button
              type="button"
              onClick={() => setActiveTab('rules')}
              aria-current={activeTab === 'rules' ? 'page' : undefined}
              className={`px-1 py-2 text-sm font-semibold border-b-2 -mb-px transition-colors ${
                activeTab === 'rules'
                  ? 'border-(--theme-border-underline-nav-active) text-(--theme-fg-default)'
                  : 'border-transparent text-(--theme-fg-muted) dark:text-(--theme-fg-muted) hover:text-(--theme-fg-default) dark:hover:text-(--theme-fg-default)'
              }`}
            >
              <span className="inline-flex items-center gap-2">
                <SlidersHorizontal className="size-4" aria-hidden="true" />
                {t('admin_alerts_tab_rules')}
              </span>
            </button>
            <button
              type="button"
              onClick={() => setActiveTab('channels')}
              aria-current={activeTab === 'channels' ? 'page' : undefined}
              className={`px-1 py-2 text-sm font-semibold border-b-2 -mb-px transition-colors ${
                activeTab === 'channels'
                  ? 'border-(--theme-border-underline-nav-active) text-(--theme-fg-default)'
                  : 'border-transparent text-(--theme-fg-muted) dark:text-(--theme-fg-muted) hover:text-(--theme-fg-default) dark:hover:text-(--theme-fg-default)'
              }`}
            >
              <span className="inline-flex items-center gap-2">
                <BellRing className="size-4" aria-hidden="true" />
                {t('admin_alerts_tab_channels')}
              </span>
            </button>
          </div>
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
            togglingId={rules.togglingId}
            onToggleEnabled={rules.toggleEnabled}
            renamingId={rules.renamingId}
            onRename={rules.rename}
            onEdit={rules.openEdit}
            onDelete={rules.deleteRule}
          />
        </Card>
      ) : (
        <AlertChannelsPanel
          channels={channels.channels}
          loading={channels.loading}
          togglingId={channels.togglingId}
          testingId={channels.testingId}
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
          onClose={rules.closeModal}
          onSuccess={rules.afterSave}
        />
      )}

      <AlertChannelModal
        isOpen={channels.isModalOpen}
        mode={channels.editingChannel ? 'edit' : 'add'}
        channelId={channels.editingChannel?.id}
        initialForm={channels.modalForm}
        isSaving={channels.saving}
        onClose={channels.closeModal}
        onSave={channels.saveChannel}
      />
    </div>
  );
};

export default AlertManager;
