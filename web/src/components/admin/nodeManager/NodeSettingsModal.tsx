import React from 'react';
import Copy from 'lucide-react/dist/esm/icons/copy';
import Check from 'lucide-react/dist/esm/icons/check';
import Plus from 'lucide-react/dist/esm/icons/plus';
import Tag from 'lucide-react/dist/esm/icons/tag';
import Terminal from 'lucide-react/dist/esm/icons/terminal';
import X from 'lucide-react/dist/esm/icons/x';
import Shield from 'lucide-react/dist/esm/icons/shield';
import Globe from 'lucide-react/dist/esm/icons/globe';
import Button from '@components/ui/Button';
import Input from '@components/ui/Input';
import IOSSwitch from '@components/ui/IOSSwitch';
import { Modal, ModalHeader, ModalBody, ModalFooter } from '@components/ui/Modal';
import { useTopBanner } from '@components/ui/TopBannerStack';
import { PlatformLogo } from '@components/system/SystemLogo';
import type { NodeRow } from '@app-types/admin';
import type { Group, NodeDeploy, NodeDeployPlatform } from '@app-types/api';
import { useI18n } from '@i18n';
import { copyTextToClipboardWithFeedback } from '@utils/clipboard';
import { normalizeTags } from '@utils/tags';

interface Props {
  isOpen: boolean;
  onClose: () => void;
  node: NodeRow;
  groups: Group[];
  deploy?: NodeDeploy | null;
  onSave: (input: {
    name: string;
    secret: string;
    guestVisible: boolean;
    groupIds: number[];
    tags?: string[];
  }) => void;
}

const platforms: { id: NodeDeployPlatform; label: string }[] = [
  { id: 'linux', label: 'Linux' },
  { id: 'windows', label: 'Windows' },
  { id: 'macos', label: 'macOS' },
];

interface TagDraft {
  id: string;
  value: string;
  kind: 'saved' | 'new';
}

let tagDraftSeq = 0;

const newTagDraft = (value = '', kind: TagDraft['kind'] = 'new'): TagDraft => {
  tagDraftSeq += 1;
  return { id: `tag-${tagDraftSeq}`, value, kind };
};

const tagDraftsFromList = (tags: string[]): TagDraft[] =>
  tags.map((tag) => newTagDraft(tag, 'saved'));
const tagListFromDrafts = (drafts: TagDraft[]): string[] =>
  normalizeTags(drafts.map((draft) => draft.value));

const NodeSettingsModal: React.FC<Props> = ({ isOpen, onClose, node, groups, deploy, onSave }) => {
  const titleId = React.useId();
  const nameId = React.useId();
  const groupsLabelId = React.useId();
  const tagsLabelId = React.useId();
  const secretId = React.useId();
  const [name, setName] = React.useState(node.name);
  const [secret, setSecret] = React.useState(node.secret);
  const [guestVisible, setGuestVisible] = React.useState(node.guestVisible);
  const [selectedGroups, setSelectedGroups] = React.useState<number[]>(node.groupIds);
  const [tagDrafts, setTagDrafts] = React.useState<TagDraft[]>(() => tagDraftsFromList(node.tags));
  const [editingTagIds, setEditingTagIds] = React.useState<Set<string>>(() => new Set());
  const [tagsDirty, setTagsDirty] = React.useState(false);
  const [activePlatform, setActivePlatform] = React.useState<NodeDeployPlatform>('linux');
  const [copied, setCopied] = React.useState(false);
  const tagInputRefs = React.useRef(new Map<string, HTMLInputElement>());
  const pendingTagFocusId = React.useRef<string | null>(null);
  const pushBanner = useTopBanner();
  const { t } = useI18n();
  const installCommand = React.useMemo(() => {
    const prefix = deploy?.scripts?.[activePlatform]?.command_prefix;
    if (!prefix) return null;
    return `${prefix}${secret}`;
  }, [activePlatform, deploy, secret]);

  React.useEffect(() => {
    setName(node.name);
    setSecret(node.secret);
    setGuestVisible(node.guestVisible);
    setSelectedGroups(node.groupIds);
    setTagDrafts(tagDraftsFromList(node.tags));
    setEditingTagIds(new Set());
    setTagsDirty(false);
    pendingTagFocusId.current = null;
  }, [node]);

  React.useLayoutEffect(() => {
    const id = pendingTagFocusId.current;
    if (!id) return;
    const input = tagInputRefs.current.get(id);
    if (!input) return;
    input.focus();
    input.select();
    pendingTagFocusId.current = null;
  }, [tagDrafts, editingTagIds]);

  const setTagInputRef = React.useCallback(
    (id: string) => (input: HTMLInputElement | null) => {
      if (input) {
        tagInputRefs.current.set(id, input);
      } else {
        tagInputRefs.current.delete(id);
      }
    },
    [],
  );

  const toggleGroup = (groupId: number) => {
    setSelectedGroups((prev) =>
      prev.includes(groupId) ? prev.filter((id) => id !== groupId) : [...prev, groupId],
    );
  };

  const changeTag = (id: string, value: string) => {
    setTagsDirty(true);
    setTagDrafts((prev) => prev.map((draft) => (draft.id === id ? { ...draft, value } : draft)));
  };

  const addTag = () => {
    const draft = newTagDraft();
    pendingTagFocusId.current = draft.id;
    setTagsDirty(true);
    setTagDrafts((prev) => [...prev, draft]);
  };

  const editTag = (id: string) => {
    pendingTagFocusId.current = id;
    setEditingTagIds((prev) => {
      if (prev.has(id)) return prev;
      const next = new Set(prev);
      next.add(id);
      return next;
    });
  };

  const removeTag = (id: string) => {
    setTagsDirty(true);
    setEditingTagIds((prev) => {
      if (!prev.has(id)) return prev;
      const next = new Set(prev);
      next.delete(id);
      return next;
    });
    setTagDrafts((prev) => prev.filter((draft) => draft.id !== id));
  };

  const save = () => {
    const input: {
      name: string;
      secret: string;
      guestVisible: boolean;
      groupIds: number[];
      tags?: string[];
    } = {
      name: name.trim() || node.name,
      secret: secret.trim(),
      guestVisible,
      groupIds: selectedGroups,
    };
    if (tagsDirty) {
      input.tags = tagListFromDrafts(tagDrafts);
    }
    onSave(input);
    onClose();
  };

  const copyCommand = React.useCallback(async () => {
    if (!installCommand) {
      pushBanner(t('admin_deploy_command_unavailable'), { tone: 'error' });
      return;
    }

    const ok = await copyTextToClipboardWithFeedback(installCommand, {
      pushBanner,
      successMessage: t('admin_deploy_command_copied'),
      httpsRequiredMessage: t('admin_clipboard_https_required'),
      failureMessage: t('admin_copy_failed_manual'),
    });
    if (!ok) return;

    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  }, [installCommand, pushBanner, t]);

  const savedTagDrafts = tagDrafts.filter(
    (draft) => draft.kind === 'saved' && !editingTagIds.has(draft.id),
  );
  const editableTagDrafts = tagDrafts.filter(
    (draft) => draft.kind === 'new' || editingTagIds.has(draft.id),
  );

  if (!isOpen) return null;

  return (
    <Modal isOpen={isOpen} onClose={onClose} maxWidth="max-w-2xl" ariaLabelledby={titleId}>
      <ModalHeader title={t('admin_node_settings_title')} onClose={onClose} id={titleId} />

      <ModalBody className="space-y-8">
        <div className="space-y-4">
          <h3 className="text-sm font-semibold text-(--theme-fg-default) dark:text-(--theme-fg-strong) uppercase tracking-wider mb-3">
            {t('admin_node_settings_basic')}
          </h3>

          <div className="space-y-4">
            <div className="space-y-1.5">
              <label
                htmlFor={nameId}
                className="text-xs font-medium text-(--theme-fg-muted) dark:text-(--theme-fg-strong) ml-1"
              >
                {t('admin_node_settings_name')}
              </label>
              <Input
                id={nameId}
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder={t('admin_node_settings_name_placeholder')}
                className="dark:bg-(--theme-bg-inset)"
              />
            </div>

            <div className="space-y-1.5">
              <label
                id={groupsLabelId}
                className="text-xs font-medium text-(--theme-fg-muted) dark:text-(--theme-fg-strong) ml-1"
              >
                {t('admin_node_settings_groups')}
              </label>
              {groups.length === 0 ? (
                <div className="text-xs text-(--theme-fg-muted) dark:text-(--theme-fg-strong) px-3 py-2 rounded-xl border border-dashed border-(--theme-border-hover) dark:border-(--theme-border-default) bg-(--theme-bg-muted) dark:bg-(--theme-bg-inset)">
                  {t('admin_node_settings_no_groups', { tab: t('admin_tab_groups') })}
                </div>
              ) : (
                <div
                  className="grid grid-cols-1 sm:grid-cols-2 gap-2 max-h-40 overflow-y-auto rounded-xl border border-(--theme-border-subtle) dark:border-(--theme-border-default) bg-(--theme-bg-default) dark:bg-(--theme-bg-inset) p-3"
                  aria-labelledby={groupsLabelId}
                >
                  {groups.map((group) => (
                    <label
                      key={group.id}
                      className="flex items-center gap-2 text-sm text-(--theme-fg-default) dark:text-(--theme-fg-strong) cursor-pointer"
                    >
                      <input
                        type="checkbox"
                        className="rounded border-(--theme-border-hover) dark:border-(--theme-border-default)/60 text-(--theme-fg-interactive-strong) focus:ring-(--theme-fg-interactive)"
                        checked={selectedGroups.includes(group.id)}
                        onChange={() => toggleGroup(group.id)}
                      />
                      <span className="truncate">{group.name}</span>
                    </label>
                  ))}
                </div>
              )}
            </div>
          </div>

          <div className="space-y-1.5">
            <div className="ml-1 flex items-center justify-between gap-3">
              <label
                id={tagsLabelId}
                className="flex items-center gap-1.5 text-xs font-medium text-(--theme-fg-muted) dark:text-(--theme-fg-strong)"
              >
                <Tag size={12} /> {t('admin_node_settings_tags')}
              </label>
              <button
                type="button"
                className="ui-focus-ring inline-flex size-6 shrink-0 items-center justify-center rounded-md text-(--theme-fg-muted) transition-colors hover:bg-(--theme-bg-muted) hover:text-(--theme-fg-default) dark:hover:bg-(--theme-bg-default)"
                onClick={addTag}
                aria-label={t('admin_node_settings_add_tag')}
                title={t('admin_node_settings_add_tag')}
              >
                <Plus size={14} />
              </button>
            </div>
            <div className="space-y-2" aria-labelledby={tagsLabelId}>
              {savedTagDrafts.length === 0 && editableTagDrafts.length === 0 ? (
                <p className="px-1 text-xs text-(--theme-fg-muted) dark:text-(--theme-fg-neutral)">
                  {t('admin_node_settings_tags_empty')}
                </p>
              ) : null}

              {savedTagDrafts.length > 0 ? (
                <div className="flex flex-wrap items-start gap-2">
                  {savedTagDrafts.map((draft) => (
                    <div
                      key={draft.id}
                      className="inline-flex max-w-full items-center gap-1 rounded-md border border-(--theme-border-subtle) bg-(--theme-bg-muted) px-2 py-1 text-xs text-(--theme-fg-default) shadow-sm dark:border-(--theme-border-default) dark:bg-(--theme-bg-default)"
                      title={draft.value}
                    >
                      <button
                        type="button"
                        className="ui-focus-ring min-w-0 rounded px-0.5 text-left"
                        onClick={() => editTag(draft.id)}
                      >
                        <span className="block truncate whitespace-nowrap">{draft.value}</span>
                      </button>
                      <button
                        type="button"
                        className="ui-focus-ring -mr-1 inline-flex size-5 shrink-0 items-center justify-center rounded text-(--theme-fg-muted) transition-colors hover:bg-(--theme-bg-danger-subtle) hover:text-(--theme-fg-danger) dark:hover:bg-(--theme-bg-danger-muted)"
                        onClick={() => removeTag(draft.id)}
                        aria-label={t('admin_node_settings_remove_tag')}
                        title={t('admin_node_settings_remove_tag')}
                      >
                        <X size={13} />
                      </button>
                    </div>
                  ))}
                </div>
              ) : null}

              {editableTagDrafts.length > 0 ? (
                <div className="space-y-2">
                  {editableTagDrafts.map((draft) => (
                    <div key={draft.id} className="flex items-center gap-2">
                      <Input
                        ref={setTagInputRef(draft.id)}
                        value={draft.value}
                        onChange={(event) => changeTag(draft.id, event.target.value)}
                        placeholder={t('admin_node_settings_tag_placeholder')}
                        aria-label={t('admin_node_settings_tag_placeholder')}
                        wrapperClassName="min-w-0 flex-1"
                        className="dark:bg-(--theme-bg-inset)"
                      />
                      <button
                        type="button"
                        className="ui-focus-ring inline-flex size-8 shrink-0 items-center justify-center rounded-md text-(--theme-fg-muted) transition-colors hover:bg-(--theme-bg-danger-subtle) hover:text-(--theme-fg-danger) dark:hover:bg-(--theme-bg-danger-muted)"
                        onClick={() => removeTag(draft.id)}
                        aria-label={t('admin_node_settings_remove_tag')}
                        title={t('admin_node_settings_remove_tag')}
                      >
                        <X size={15} />
                      </button>
                    </div>
                  ))}
                </div>
              ) : null}
            </div>
          </div>

          <div className="space-y-1.5">
            <label
              htmlFor={secretId}
              className="text-xs font-medium text-(--theme-fg-muted) dark:text-(--theme-fg-strong) ml-1 flex items-center gap-1.5"
            >
              <Shield size={12} /> {t('admin_node_settings_secret_label')}
            </label>
            <div className="relative">
              <Input
                id={secretId}
                value={secret}
                onChange={(e) => setSecret(e.target.value)}
                className="font-mono text-sm pr-10 dark:bg-(--theme-bg-inset)"
              />
            </div>
            <p className="text-[11px] text-(--theme-fg-subtle) ml-1">
              {t('admin_node_settings_secret_hint')}
            </p>
          </div>

          <div className="flex items-center justify-between p-4 rounded-xl border border-(--theme-border-subtle) dark:border-(--theme-border-default) bg-(--theme-surface-info)/70 dark:bg-(--theme-bg-inset)">
            <div className="flex items-center gap-3">
              <div className="flex size-8 shrink-0 items-center justify-center text-(--theme-fg-success)">
                <Globe size={18} />
              </div>
              <div>
                <div className="text-sm font-medium text-(--theme-fg-default) dark:text-(--theme-fg-default)">
                  {t('admin_node_settings_guest_visible')}
                </div>
                <div className="text-xs text-(--theme-fg-muted) dark:text-(--theme-fg-neutral)">
                  {t('admin_node_settings_guest_visible_desc')}
                </div>
              </div>
            </div>
            <IOSSwitch checked={guestVisible} onChange={() => setGuestVisible((prev) => !prev)} />
          </div>
        </div>

        <div className="space-y-4">
          <h3 className="text-sm font-semibold text-(--theme-fg-default) dark:text-(--theme-fg-strong) uppercase tracking-wider mb-3 flex items-center gap-2">
            <Terminal size={16} /> {t('admin_node_settings_deploy')}
          </h3>

          <div className="rounded-xl overflow-hidden ring-1 ring-(--theme-border-default) shadow-xl">
            <div className="flex border-b border-(--theme-border-default)">
              {platforms.map((p) => (
                <button
                  key={p.id}
                  onClick={() => setActivePlatform(p.id)}
                  className={`flex-1 flex items-center justify-center gap-2 py-3 text-sm font-medium transition-colors ${
                    activePlatform === p.id
                      ? 'bg-(--theme-bg-inverse) text-(--theme-fg-inverse)'
                      : 'text-(--theme-fg-subtle) hover:text-(--theme-fg-default) hover:bg-(--theme-bg-default)/50'
                  }`}
                >
                  <PlatformLogo platform={p.id} size={16} />
                  {p.label}
                </button>
              ))}
            </div>

            <div
              className="p-4 group cursor-pointer relative"
              onClick={copyCommand}
              onKeyDown={(event) => {
                if (event.key === 'Enter' || event.key === ' ') {
                  event.preventDefault();
                  copyCommand();
                }
              }}
              role="button"
              tabIndex={0}
              aria-label={t('admin_node_settings_click_copy')}
            >
              <code className="block font-mono text-xs/relaxed sm:text-sm text-(--theme-fg-command) break-all">
                {installCommand ?? t('admin_deploy_command_unavailable')}
              </code>

              <div className="absolute top-3 right-3 opacity-0 group-hover:opacity-100 group-focus-within:opacity-100 transition-opacity">
                <div className="bg-(--theme-canvas-muted) text-(--theme-fg-on-emphasis) text-xs px-2 py-1 rounded shadow-lg flex items-center gap-1">
                  {copied ? <Check size={12} /> : <Copy size={12} />}
                  {copied ? t('admin_node_settings_copied') : t('admin_node_settings_click_copy')}
                </div>
              </div>
            </div>
          </div>
        </div>
      </ModalBody>

      <ModalFooter>
        <Button variant="secondary" onClick={onClose}>
          {t('common_cancel')}
        </Button>
        <Button variant="primary" onClick={save}>
          {t('common_save_changes')}
        </Button>
      </ModalFooter>
    </Modal>
  );
};

export default NodeSettingsModal;
