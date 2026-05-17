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
import { useTopBanner } from '@components/ui/TopBannerStack';
import { Modal, ModalHeader, ModalBody, ModalFooter } from '@components/ui/Modal';
import ConfirmDialog from '@components/ui/ConfirmDialog';
import { useAuth } from '@context/AuthContext';
import { createGroup, deleteGroup, fetchGroupList, updateGroup } from '@lib/adminApi';
import type { Group } from '@app-types/api';
import { useI18n } from '@i18n';
import { useApiErrorHandler } from '@hooks/useApiErrorHandler';

const GroupManager: React.FC = () => {
  const { token } = useAuth();
  const { t } = useI18n();
  const apiError = useApiErrorHandler();
  const titleId = React.useId();
  const nameId = React.useId();
  const remarkId = React.useId();
  const [groups, setGroups] = React.useState<Group[]>([]);
  const [search, setSearch] = React.useState('');
  const [isModalOpen, setIsModalOpen] = React.useState(false);
  const [editingGroup, setEditingGroup] = React.useState<Group | null>(null);
  const [deleteTarget, setDeleteTarget] = React.useState<Group | null>(null);
  const [isLoading, setIsLoading] = React.useState(true);
  const [isSaving, setIsSaving] = React.useState(false);
  const [isDeleting, setIsDeleting] = React.useState(false);
  const pushBanner = useTopBanner();

  const [draft, setDraft] = React.useState({ name: '', remark: '' });

  const refreshGroups = React.useCallback(async () => {
    if (!token) return;
    setIsLoading(true);
    try {
      const nextGroups = await fetchGroupList();
      setGroups(nextGroups);
    } catch (error) {
      apiError(error, t('admin_groups_fetch_failed'));
    } finally {
      setIsLoading(false);
    }
  }, [apiError, token, t]);

  React.useEffect(() => {
    refreshGroups();
  }, [refreshGroups]);

  const filteredGroups = React.useMemo(() => {
    const keyword = search.trim().toLowerCase();
    if (!keyword) return groups;
    return groups.filter(
      (g) =>
        g.name.toLowerCase().includes(keyword) ||
        (g.remark && g.remark.toLowerCase().includes(keyword)),
    );
  }, [groups, search]);

  const open = (group?: Group) => {
    if (group) {
      setEditingGroup(group);
      setDraft({ name: group.name, remark: group.remark || '' });
    } else {
      setEditingGroup(null);
      setDraft({ name: '', remark: '' });
    }
    setIsModalOpen(true);
  };

  const close = () => {
    setIsModalOpen(false);
    setEditingGroup(null);
    setDraft({ name: '', remark: '' });
    setIsSaving(false);
  };

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!draft.name.trim()) {
      pushBanner(t('admin_groups_form_name_required'), { tone: 'warning' });
      return;
    }
    if (!token) return;

    setIsSaving(true);
    const input = {
      name: draft.name.trim(),
      remark: draft.remark.trim() || undefined,
    };

    try {
      if (editingGroup) {
        await updateGroup(editingGroup.id, input);
        pushBanner(t('admin_group_updated'), { tone: 'info' });
      } else {
        await createGroup(input);
        pushBanner(t('admin_group_created'), { tone: 'info' });
      }
      await refreshGroups();
      close();
    } catch (error) {
      apiError(
        error,
        editingGroup ? t('admin_group_update_failed') : t('admin_group_create_failed'),
      );
    } finally {
      setIsSaving(false);
    }
  };

  const askDelete = (group: Group) => {
    setDeleteTarget(group);
  };

  const remove = async () => {
    if (!deleteTarget || !token) return;
    setIsDeleting(true);
    try {
      await deleteGroup(deleteTarget.id);
      pushBanner(t('admin_group_deleted'), { tone: 'info' });
      await refreshGroups();
      setDeleteTarget(null);
    } catch (error) {
      apiError(error, t('admin_group_delete_failed'));
    } finally {
      setIsDeleting(false);
    }
  };

  return (
    <div className="space-y-4 md:space-y-6 animate-in fade-in duration-500">
      <div className="flex flex-col md:flex-row justify-between gap-3 md:gap-4">
        <div className="flex w-full md:w-auto md:flex-1 gap-2">
          <Input
            icon={Search}
            placeholder={t('admin_groups_search_placeholder')}
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            data-search-input="true"
            wrapperClassName="flex-1 max-w-md"
          />
        </div>

        <div className="flex w-full md:w-auto gap-2">
          <Button
            onClick={() => open()}
            icon={Plus}
            className="w-full md:w-auto shadow-(color:--theme-shadow-interactive)"
          >
            {t('admin_groups_new')}
          </Button>
        </div>
      </div>

      {isLoading ? (
        <Card className="p-8 text-center text-(--theme-fg-muted) dark:text-(--theme-fg-neutral)">
          {t('admin_groups_loading')}
        </Card>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
          {filteredGroups.map((group) => (
            <Card
              key={group.id}
              className="group relative overflow-hidden hover:shadow-lg transition-[box-shadow,border-color,background-color] duration-300 border-(--theme-border-subtle) dark:border-(--theme-border-default) bg-(--theme-bg-default) dark:bg-(--theme-bg-default)"
            >
              <div className="absolute top-0 right-0 p-4 flex gap-2 opacity-100 pointer-events-auto transition-opacity md:opacity-0 md:pointer-events-none md:group-hover:opacity-100 md:group-hover:pointer-events-auto md:group-focus-within:opacity-100 md:group-focus-within:pointer-events-auto">
                <button
                  type="button"
                  onClick={() => open(group)}
                  className="p-2 text-(--theme-fg-subtle) hover:text-(--theme-fg-interactive) hover:bg-(--theme-bg-interactive-muted) dark:hover:bg-(--theme-bg-interactive-soft) rounded-lg transition-colors"
                  title={t('admin_groups_edit')}
                  aria-label={t('admin_groups_edit')}
                >
                  <Edit2 size={16} />
                </button>
                {group.id !== 1 && (
                  <button
                    type="button"
                    onClick={() => askDelete(group)}
                    className="p-2 text-(--theme-fg-subtle) hover:text-(--theme-fg-interactive) hover:bg-(--theme-bg-interactive-muted) dark:hover:bg-(--theme-bg-interactive-soft) rounded-lg transition-colors"
                    title={t('admin_groups_delete')}
                    aria-label={t('admin_groups_delete')}
                  >
                    <Trash2 size={16} />
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
                  <div
                    className="flex items-center gap-2"
                    title={t('admin_groups_associated_servers')}
                  >
                    <Server size={16} />
                    <span>{t('admin_groups_servers_count', { count: group.server_count })}</span>
                  </div>
                </div>
              </div>
            </Card>
          ))}

          {filteredGroups.length === 0 && (
            <div className="col-span-full py-12 text-center text-(--theme-fg-muted) dark:text-(--theme-fg-neutral) bg-(--theme-bg-muted) dark:bg-(--theme-canvas-subtle) rounded-2xl border border-dashed border-(--theme-border-subtle) dark:border-(--theme-border-default)">
              <Users size={48} className="mx-auto mb-4 opacity-20" />
              <p className="text-lg font-medium">{t('admin_groups_empty_title')}</p>
              <p className="text-sm mt-1">{t('admin_groups_empty_subtitle')}</p>
            </div>
          )}
        </div>
      )}

      <ConfirmDialog
        isOpen={!!deleteTarget}
        title={t('admin_groups_delete_title')}
        message={t('admin_groups_delete_confirm', { name: deleteTarget?.name || '' })}
        confirmLabel={isDeleting ? t('admin_groups_deleting') : t('admin_groups_delete')}
        cancelLabel={t('common_cancel')}
        tone="danger"
        isLoading={isDeleting}
        onConfirm={remove}
        onCancel={() => setDeleteTarget(null)}
      />

      <Modal isOpen={isModalOpen} onClose={close} maxWidth="max-w-md" ariaLabelledby={titleId}>
        <ModalHeader
          title={
            editingGroup ? t('admin_groups_modal_edit_title') : t('admin_groups_modal_new_title')
          }
          onClose={close}
          id={titleId}
        />
        <form onSubmit={submit} className="flex flex-col flex-1 overflow-hidden">
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
                value={draft.name}
                onChange={(e) => setDraft({ ...draft, name: e.target.value })}
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
                value={draft.remark}
                onChange={(e) => setDraft({ ...draft, remark: e.target.value })}
              />
            </div>
          </ModalBody>

          <ModalFooter>
            <Button type="button" variant="secondary" onClick={close} disabled={isSaving}>
              {t('common_cancel')}
            </Button>
            <Button type="submit" disabled={isSaving}>
              {editingGroup
                ? isSaving
                  ? t('admin_groups_saving')
                  : t('admin_groups_save_changes')
                : isSaving
                  ? t('admin_groups_creating')
                  : t('admin_groups_create')}
            </Button>
          </ModalFooter>
        </form>
      </Modal>
    </div>
  );
};

export default GroupManager;
