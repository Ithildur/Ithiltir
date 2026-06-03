import React from 'react';
import Edit2 from 'lucide-react/dist/esm/icons/edit-2';
import Plus from 'lucide-react/dist/esm/icons/plus';
import Search from 'lucide-react/dist/esm/icons/search';
import Trash2 from 'lucide-react/dist/esm/icons/trash-2';
import Users from 'lucide-react/dist/esm/icons/users';
import Folder from 'lucide-react/dist/esm/icons/folder';
import Server from 'lucide-react/dist/esm/icons/server';
import Button from '@components/ui/Button';
import Card from '@components/ui/Card';
import Input from '@components/ui/Input';
import SearchInput from '@components/ui/SearchInput';
import { Modal, ModalBody, ModalFooter, ModalHeader } from '@components/ui/Modal';
import ConfirmDialog from '@components/ui/ConfirmDialog';
import type { Group } from '@app-types/api';
import { useI18n, type I18nValue } from '@i18n';
import { useGroupManager } from './useGroupManager';

type Translate = I18nValue['t'];
type GroupModalState = ReturnType<typeof useGroupManager>['modal'];
type GroupDeleteDialogState = ReturnType<typeof useGroupManager>['deleteDialog'];

const GroupCard = ({
  group,
  t,
  onEdit,
  onDelete,
}: {
  group: Group;
  t: Translate;
  onEdit: (group: Group) => void;
  onDelete: (group: Group) => void;
}) => (
  <Card className="group relative overflow-hidden hover:shadow-lg transition-[box-shadow,border-color,background-color] duration-300 border-(--theme-border-subtle) dark:border-(--theme-border-default) bg-(--theme-bg-default) dark:bg-(--theme-bg-default)">
    <div className="absolute top-0 right-0 p-4 flex gap-2 opacity-100 pointer-events-auto transition-opacity md:opacity-0 md:pointer-events-none md:group-hover:opacity-100 md:group-hover:pointer-events-auto md:group-focus-within:opacity-100 md:group-focus-within:pointer-events-auto">
      <button
        type="button"
        onClick={() => onEdit(group)}
        className="p-2 text-(--theme-fg-subtle) hover:text-(--theme-fg-interactive) hover:bg-(--theme-bg-interactive-muted) dark:hover:bg-(--theme-bg-interactive-soft) rounded-lg transition-colors"
        title={t('admin_groups_edit')}
        aria-label={t('admin_groups_edit')}
      >
        <Edit2 size={18} />
      </button>
      {group.id !== 1 && (
        <button
          type="button"
          onClick={() => onDelete(group)}
          className="p-2 text-(--theme-fg-subtle) hover:text-(--theme-fg-interactive) hover:bg-(--theme-bg-interactive-muted) dark:hover:bg-(--theme-bg-interactive-soft) rounded-lg transition-colors"
          title={t('admin_groups_delete')}
          aria-label={t('admin_groups_delete')}
        >
          <Trash2 size={18} />
        </button>
      )}
    </div>

    <div className="p-6 space-y-4">
      <div className="flex items-start gap-4">
        <div className="flex size-10 shrink-0 items-center justify-center text-black dark:text-white">
          <Folder size={24} strokeWidth={1.75} />
        </div>
        <div className="flex-1 min-w-0 pt-1">
          <h3 className="font-semibold text-lg text-(--theme-fg-default) dark:text-(--theme-fg-strong) truncate">
            {group.name}
          </h3>
          <p className="text-sm text-(--theme-fg-muted) dark:text-(--theme-fg-neutral) line-clamp-2 mt-1 h-10">
            {group.remark || t('admin_groups_no_description')}
          </p>
        </div>
      </div>

      <div className="pt-4 border-t border-(--theme-border-muted) dark:border-(--theme-border-default) flex items-end justify-between text-sm text-(--theme-fg-muted) dark:text-(--theme-fg-neutral)">
        <div className="flex items-center gap-2" title={t('admin_groups_associated_servers')}>
          <Server size={16} />
          <span>{t('admin_groups_servers_count', { count: group.server_count })}</span>
        </div>
      </div>
    </div>
  </Card>
);

const GroupEmptyState = ({ t }: { t: Translate }) => (
  <div className="col-span-full py-12 text-center text-(--theme-fg-muted) dark:text-(--theme-fg-neutral) bg-(--theme-bg-muted) dark:bg-(--theme-canvas-subtle) rounded-2xl border border-dashed border-(--theme-border-subtle) dark:border-(--theme-border-default)">
    <Users size={48} className="mx-auto mb-4 opacity-20" />
    <p className="text-lg font-medium">{t('admin_groups_empty_title')}</p>
    <p className="text-sm mt-1">{t('admin_groups_empty_subtitle')}</p>
  </div>
);

const GroupDeleteDialog = ({ dialog, t }: { dialog: GroupDeleteDialogState; t: Translate }) => (
  <ConfirmDialog
    isOpen={dialog.target !== null}
    title={t('admin_groups_delete_title')}
    message={t('admin_groups_delete_confirm', { name: dialog.target?.name || '' })}
    confirmLabel={dialog.isDeleting ? t('admin_groups_deleting') : t('admin_groups_delete')}
    cancelLabel={t('common_cancel')}
    tone="danger"
    isLoading={dialog.isDeleting}
    onConfirm={dialog.confirm}
    onCancel={dialog.close}
  />
);

const GroupFormModal = ({ modal, t }: { modal: GroupModalState; t: Translate }) => {
  const titleId = React.useId();
  const nameId = React.useId();
  const remarkId = React.useId();

  return (
    <Modal isOpen={modal.isOpen} onClose={modal.close} maxWidth="max-w-md" ariaLabelledby={titleId}>
      <ModalHeader
        title={
          modal.editingGroup
            ? t('admin_groups_modal_edit_title')
            : t('admin_groups_modal_new_title')
        }
        onClose={modal.close}
        id={titleId}
      />
      <form onSubmit={modal.submit} className="flex flex-col flex-1 overflow-hidden">
        <ModalBody className="space-y-4">
          <div className="space-y-2">
            <label
              htmlFor={nameId}
              className="text-sm font-medium text-(--theme-fg-default) dark:text-(--theme-fg-control-hover)"
            >
              {t('admin_groups_form_name')}{' '}
              <span className="text-(--theme-fg-danger-muted)">*</span>
            </label>
            <Input
              id={nameId}
              value={modal.draft.name}
              onChange={(event) => modal.patchDraft({ name: event.target.value })}
              placeholder={t('admin_groups_form_name_placeholder')}
              autoFocus
            />
          </div>

          <div className="space-y-2">
            <label
              htmlFor={remarkId}
              className="text-sm font-medium text-(--theme-fg-default) dark:text-(--theme-fg-control-hover)"
            >
              {t('admin_groups_form_desc')}
            </label>
            <textarea
              id={remarkId}
              className="w-full px-3 py-2 bg-(--theme-bg-default) dark:bg-(--theme-bg-default) border border-(--theme-border-subtle) dark:border-(--theme-border-default) rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-(--theme-fg-interactive)/20 focus:border-(--theme-fg-interactive) transition-[background-color,border-color,box-shadow] placeholder-(--theme-fg-subtle) dark:text-(--theme-fg-default) resize-none h-24"
              placeholder={t('admin_groups_form_desc_placeholder')}
              value={modal.draft.remark}
              onChange={(event) => modal.patchDraft({ remark: event.target.value })}
            />
          </div>
        </ModalBody>

        <ModalFooter>
          <Button type="button" variant="secondary" onClick={modal.close} disabled={modal.saving}>
            {t('common_cancel')}
          </Button>
          <Button type="submit" disabled={modal.saving}>
            {modal.editingGroup
              ? modal.saving
                ? t('admin_groups_saving')
                : t('admin_groups_save_changes')
              : modal.saving
                ? t('admin_groups_creating')
                : t('admin_groups_create')}
          </Button>
        </ModalFooter>
      </form>
    </Modal>
  );
};

const GroupManager: React.FC = () => {
  const { t } = useI18n();
  const model = useGroupManager();

  return (
    <div className="space-y-4 md:space-y-6 animate-in fade-in duration-500">
      <div className="flex flex-col md:flex-row justify-between gap-3 md:gap-4">
        <div className="flex w-full md:w-auto md:flex-1 gap-2">
          <SearchInput
            icon={Search}
            placeholder={t('admin_groups_search_placeholder')}
            value={model.search}
            onChange={(event) => model.setSearch(event.target.value)}
            wrapperClassName="flex-1 max-w-md"
          />
        </div>

        <div className="flex w-full md:w-auto gap-2">
          <Button
            onClick={() => model.modal.open()}
            icon={Plus}
            className="w-full md:w-auto shadow-(color:--theme-shadow-interactive)"
          >
            {t('admin_groups_new')}
          </Button>
        </div>
      </div>

      {model.isLoading ? (
        <Card className="p-8 text-center text-(--theme-fg-muted) dark:text-(--theme-fg-neutral)">
          {t('admin_groups_loading')}
        </Card>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
          {model.groups.map((group) => (
            <GroupCard
              key={group.id}
              group={group}
              t={t}
              onEdit={model.modal.open}
              onDelete={model.deleteDialog.open}
            />
          ))}
          {model.groups.length === 0 && <GroupEmptyState t={t} />}
        </div>
      )}

      <GroupDeleteDialog dialog={model.deleteDialog} t={t} />
      <GroupFormModal modal={model.modal} t={t} />
    </div>
  );
};

export default GroupManager;
