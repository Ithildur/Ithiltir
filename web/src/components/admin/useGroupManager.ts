import React from 'react';
import type { Group } from '@app-types/api';
import { useApiErrorHandler } from '@hooks/useApiErrorHandler';
import { useI18n } from '@i18n';
import { pushTopBanner } from '@runtime/topBannerRuntime';
import {
  loadAdminGroups,
  removeAdminGroup,
  saveAdminGroup,
  useAdminGroupsStore,
} from '@stores/adminGroupsStore';
import { useAuthStore } from '@stores/authStore';
import { isCanceledRequestError } from '@utils/errors';

type GroupDraft = {
  name: string;
  remark: string;
};

const emptyDraft: GroupDraft = { name: '', remark: '' };

const draftFromGroup = (group: Group): GroupDraft => ({
  name: group.name,
  remark: group.remark || '',
});

export const useGroupManager = () => {
  const token = useAuthStore((state) => state.accessToken);
  const { t } = useI18n();
  const apiError = useApiErrorHandler();
  const groups = useAdminGroupsStore((state) => state.groups);
  const isLoading = useAdminGroupsStore((state) => state.loading);
  const isSaving = useAdminGroupsStore((state) => state.saving);
  const isDeleting = useAdminGroupsStore((state) => state.deleting);
  const [search, setSearch] = React.useState('');
  const [hasLoaded, setHasLoaded] = React.useState(false);
  const [isModalOpen, setIsModalOpen] = React.useState(false);
  const [editingGroupId, setEditingGroupId] = React.useState<number | null>(null);
  const [deleteGroupId, setDeleteGroupId] = React.useState<number | null>(null);
  const [draft, setDraft] = React.useState<GroupDraft>(emptyDraft);

  const editingGroup = React.useMemo(
    () => groups.find((group) => group.id === editingGroupId) ?? null,
    [editingGroupId, groups],
  );
  const deleteTarget = React.useMemo(
    () => groups.find((group) => group.id === deleteGroupId) ?? null,
    [deleteGroupId, groups],
  );

  const refreshGroups = React.useCallback(
    async (params: { signal?: AbortSignal } = {}) => {
      if (!token) return;
      try {
        await loadAdminGroups(params);
      } catch (error) {
        if (isCanceledRequestError(error)) return;
        apiError(error, { key: 'admin_groups_fetch_failed' });
      }
    },
    [apiError, token],
  );

  React.useEffect(() => {
    if (!token) {
      setHasLoaded(false);
      return;
    }
    const controller = new AbortController();
    setHasLoaded(false);
    void refreshGroups({ signal: controller.signal }).finally(() => {
      if (!controller.signal.aborted) setHasLoaded(true);
    });
    return () => {
      controller.abort();
    };
  }, [refreshGroups, token]);

  React.useEffect(() => {
    if (isModalOpen && editingGroupId !== null && !editingGroup && !isLoading && !isSaving) {
      setIsModalOpen(false);
      setEditingGroupId(null);
      setDraft(emptyDraft);
    }
  }, [editingGroup, editingGroupId, isLoading, isModalOpen, isSaving]);

  React.useEffect(() => {
    if (deleteGroupId !== null && !deleteTarget && !isLoading && !isDeleting) {
      setDeleteGroupId(null);
    }
  }, [deleteGroupId, deleteTarget, isDeleting, isLoading]);

  const filteredGroups = React.useMemo(() => {
    const keyword = search.trim().toLowerCase();
    if (!keyword) return groups;
    return groups.filter(
      (group) =>
        group.name.toLowerCase().includes(keyword) ||
        (group.remark && group.remark.toLowerCase().includes(keyword)),
    );
  }, [groups, search]);

  const openModal = React.useCallback((group?: Group) => {
    setDraft(group ? draftFromGroup(group) : emptyDraft);
    setEditingGroupId(group?.id ?? null);
    setIsModalOpen(true);
  }, []);

  const closeModal = React.useCallback(() => {
    setIsModalOpen(false);
    setEditingGroupId(null);
    setDraft(emptyDraft);
  }, []);

  const patchDraft = React.useCallback((patch: Partial<GroupDraft>) => {
    setDraft((current) => ({ ...current, ...patch }));
  }, []);

  const submit = React.useCallback(
    async (event: React.FormEvent) => {
      event.preventDefault();
      const name = draft.name.trim();
      if (!name) {
        pushTopBanner(t('admin_groups_form_name_required'), { tone: 'warning' });
        return;
      }
      if (!token) return;

      try {
        const didSave = await saveAdminGroup(editingGroupId, {
          name,
          remark: draft.remark.trim() || undefined,
        });
        if (!didSave) return;
        pushTopBanner(
          editingGroupId !== null ? t('admin_group_updated') : t('admin_group_created'),
          { tone: 'info' },
        );
        closeModal();
      } catch (error) {
        apiError(
          error,
          editingGroupId !== null ? t('admin_group_update_failed') : t('admin_group_create_failed'),
        );
      }
    },
    [apiError, closeModal, draft, editingGroupId, t, token],
  );

  const askDelete = React.useCallback((group: Group) => {
    setDeleteGroupId(group.id);
  }, []);

  const closeDelete = React.useCallback(() => {
    setDeleteGroupId(null);
  }, []);

  const confirmDelete = React.useCallback(async () => {
    if (deleteGroupId === null || !token) return;
    try {
      const didRemove = await removeAdminGroup(deleteGroupId);
      if (!didRemove) return;
      pushTopBanner(t('admin_group_deleted'), { tone: 'info' });
      setDeleteGroupId(null);
    } catch (error) {
      apiError(error, t('admin_group_delete_failed'));
    }
  }, [apiError, deleteGroupId, t, token]);

  return {
    groups: filteredGroups,
    search,
    setSearch,
    isLoading: isLoading || (token !== null && !hasLoaded),
    modal: {
      isOpen: isModalOpen,
      editingGroup,
      draft,
      patchDraft,
      open: openModal,
      close: closeModal,
      submit,
      saving: isSaving,
    },
    deleteDialog: {
      target: deleteTarget,
      isDeleting,
      open: askDelete,
      close: closeDelete,
      confirm: confirmDelete,
    },
  };
};
