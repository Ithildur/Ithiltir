import React from 'react';
import Gauge from 'lucide-react/dist/esm/icons/gauge';
import Button from '@components/ui/Button';
import Input from '@components/ui/Input';
import IOSSwitch from '@components/ui/IOSSwitch';
import { Modal, ModalBody, ModalFooter, ModalHeader } from '@components/ui/Modal';
import type { NodeVMSettings } from '@app-types/api';
import type { NodeRow } from '@app-types/admin';
import { useI18n } from '@i18n';

interface Props {
  node: NodeRow;
  onClose: () => void;
  onSave: (settings: NodeVMSettings) => Promise<boolean>;
}

const NodeVMSettingsModal = ({ node, onClose, onSave }: Props) => {
  const { t } = useI18n();
  const titleId = React.useId();
  const [cpu, setCPU] = React.useState(String(node.vmCPUThreshold));
  const [memory, setMemory] = React.useState(String(node.vmMemoryThreshold));
  const [memoryEnabled, setMemoryEnabled] = React.useState(node.vmMemoryEnabled);
  const [saving, setSaving] = React.useState(false);
  const validThreshold = (value: string) =>
    value.trim() !== '' &&
    Number.isInteger(Number(value)) &&
    Number(value) >= 1 &&
    Number(value) <= 100;
  const memoryThreshold = memoryEnabled ? Number(memory) : node.vmMemoryThreshold;
  const valid = validThreshold(cpu) && (!memoryEnabled || validThreshold(memory));
  const changed =
    Number(cpu) !== node.vmCPUThreshold ||
    memoryThreshold !== node.vmMemoryThreshold ||
    memoryEnabled !== node.vmMemoryEnabled;
  const close = () => {
    if (!saving) onClose();
  };
  const save = async (event: React.FormEvent) => {
    event.preventDefault();
    if (saving || !valid || !changed) return;
    setSaving(true);
    try {
      if (
        await onSave({
          vm_cpu_threshold: Number(cpu),
          vm_memory_threshold: memoryThreshold,
          vm_memory_enabled: memoryEnabled,
        })
      )
        onClose();
    } finally {
      setSaving(false);
    }
  };

  return (
    <Modal isOpen onClose={close} maxWidth="max-w-xl" ariaLabelledby={titleId}>
      <ModalHeader
        id={titleId}
        title={t('admin_node_vm_title', { name: node.name })}
        icon={<Gauge className="size-5" aria-hidden="true" />}
        onClose={close}
      />
      <form onSubmit={(event) => void save(event)}>
        <ModalBody className="space-y-5">
          <p className="text-xs/5 text-(--theme-fg-muted)">{t('admin_node_vm_hint')}</p>
          <div className="grid gap-1.5">
            <label
              htmlFor={`${titleId}-cpu`}
              className="text-xs font-semibold text-(--theme-fg-muted)"
            >
              {t('admin_node_vm_cpu')}
            </label>
            <Input
              id={`${titleId}-cpu`}
              type="number"
              min={1}
              max={100}
              step={1}
              required
              value={cpu}
              onChange={(event) => setCPU(event.target.value)}
              disabled={saving}
              aria-invalid={!validThreshold(cpu)}
              rightElement="%"
            />
          </div>
          <div className="space-y-3 border-t border-(--theme-border-subtle) pt-4">
            <div className="flex items-center justify-between gap-3">
              <span className="text-sm">{t('admin_node_vm_memory_enabled')}</span>
              <IOSSwitch
                checked={memoryEnabled}
                onChange={() => setMemoryEnabled((enabled) => !enabled)}
                disabled={saving}
                ariaLabel={t('admin_node_vm_memory_enabled')}
              />
            </div>
            <div className="grid gap-1.5">
              <label
                htmlFor={`${titleId}-memory`}
                className="text-xs font-semibold text-(--theme-fg-muted)"
              >
                {t('admin_node_vm_memory')}
              </label>
              <Input
                id={`${titleId}-memory`}
                type="number"
                min={1}
                max={100}
                step={1}
                required
                value={memory}
                onChange={(event) => setMemory(event.target.value)}
                disabled={saving || !memoryEnabled}
                aria-invalid={memoryEnabled && !validThreshold(memory)}
                rightElement="%"
              />
            </div>
          </div>
          {!valid && (
            <p role="alert" className="text-xs text-(--theme-fg-danger)">
              {t('admin_node_vm_invalid')}
            </p>
          )}
        </ModalBody>
        <ModalFooter>
          <Button type="button" variant="secondary" onClick={close} disabled={saving}>
            {t('common_cancel')}
          </Button>
          <Button type="submit" variant="primary" disabled={saving || !valid || !changed}>
            {saving ? t('admin_system_settings_saving') : t('common_save_changes')}
          </Button>
        </ModalFooter>
      </form>
    </Modal>
  );
};

export default NodeVMSettingsModal;
