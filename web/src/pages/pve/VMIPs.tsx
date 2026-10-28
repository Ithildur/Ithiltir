import React from 'react';
import { Modal, ModalBody, ModalHeader } from '@components/ui/Modal';
import { useI18n } from '@i18n';
import type { PVEVM } from '@app-types/virt';

const VMIPs = ({ vm }: { vm: PVEVM }) => {
  const { t } = useI18n();
  const [open, setOpen] = React.useState(false);
  const titleId = React.useId();
  const ips = vm.ips ?? [];

  if (ips.length === 0) return <>—</>;

  return (
    <>
      <button
        type="button"
        aria-haspopup="dialog"
        onClick={() => setOpen(true)}
        className="ui-focus-ring inline-flex max-w-full items-baseline gap-1 rounded text-left hover:text-(--theme-fg-interactive)"
      >
        <span className="truncate">{ips[0]}</span>
        {ips.length > 1 && <span className="shrink-0">(+{ips.length - 1})</span>}
      </button>
      {open && (
        <Modal isOpen onClose={() => setOpen(false)} maxWidth="max-w-xl" ariaLabelledby={titleId}>
          <ModalHeader
            id={titleId}
            title={`${t('pve_ips')} · #${vm.id}`}
            onClose={() => setOpen(false)}
          />
          <ModalBody>
            <ul className="space-y-2 font-mono text-xs">
              {ips.map((ip) => (
                <li key={ip} className="wrap-anywhere">
                  {ip}
                </li>
              ))}
            </ul>
          </ModalBody>
        </Modal>
      )}
    </>
  );
};

export default VMIPs;
