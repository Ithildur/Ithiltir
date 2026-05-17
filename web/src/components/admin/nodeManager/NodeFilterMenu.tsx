import React from 'react';
import Check from 'lucide-react/dist/esm/icons/check';
import ChevronDown from 'lucide-react/dist/esm/icons/chevron-down';
import Filter from 'lucide-react/dist/esm/icons/filter';
import X from 'lucide-react/dist/esm/icons/x';
import Button from '@components/ui/Button';
import type { Group } from '@app-types/api';
import { useI18n } from '@i18n';

interface Props {
  groups: Group[];
  selectedGroupIds: number[];
  updateableOnly: boolean;
  showVersionFilter?: boolean;
  onGroupChange: (ids: number[]) => void;
  onUpdateableOnlyChange: (value: boolean) => void;
}

const NodeFilterMenu: React.FC<Props> = ({
  groups,
  selectedGroupIds,
  updateableOnly,
  showVersionFilter = true,
  onGroupChange,
  onUpdateableOnlyChange,
}) => {
  const { t } = useI18n();
  const [isOpen, setIsOpen] = React.useState(false);
  const containerRef = React.useRef<HTMLDivElement>(null);

  React.useEffect(() => {
    const closeOnOutside = (event: MouseEvent) => {
      if (containerRef.current && !containerRef.current.contains(event.target as Node)) {
        setIsOpen(false);
      }
    };
    document.addEventListener('mousedown', closeOnOutside);
    return () => document.removeEventListener('mousedown', closeOnOutside);
  }, []);

  const activeCount = selectedGroupIds.length + (showVersionFilter && updateableOnly ? 1 : 0);

  const toggleGroup = (id: number) => {
    if (selectedGroupIds.includes(id)) {
      onGroupChange(selectedGroupIds.filter((item) => item !== id));
    } else {
      onGroupChange([...selectedGroupIds, id]);
    }
  };

  const itemClass = (selected: boolean) =>
    selected ? 'menu-item gap-3' : 'menu-item menu-item-hover gap-3';

  const checkboxClass = (selected: boolean) =>
    `flex h-5 w-5 shrink-0 items-center justify-center rounded-[3px] border transition-[background-color,border-color] ${
      selected
        ? 'border-(--theme-bg-accent-emphasis) bg-(--theme-bg-accent-emphasis) text-(--theme-fg-on-emphasis)'
        : 'border-(--theme-border-default) bg-(--theme-bg-default) dark:bg-(--theme-bg-inset)'
    }`;

  return (
    <div className="relative" ref={containerRef}>
      <Button
        type="button"
        variant="secondary"
        onClick={() => setIsOpen((open) => !open)}
        aria-haspopup="dialog"
        aria-expanded={isOpen}
        className={
          activeCount > 0 ? 'border-(--theme-fg-accent)/50 ring-1 ring-(--theme-fg-accent)/20' : ''
        }
      >
        <Filter size={16} className={activeCount > 0 ? 'text-(--theme-fg-accent)' : ''} />
        <span className="hidden sm:inline">{t('admin_nodes_filter')}</span>
        {activeCount > 0 && (
          <span className="flex h-5 min-w-5 items-center justify-center rounded-full bg-(--theme-bg-accent-muted) px-1 text-xs font-bold text-(--theme-fg-accent)">
            {activeCount}
          </span>
        )}
        <ChevronDown size={12} className="text-(--theme-fg-muted)" />
      </Button>

      {isOpen && (
        <div
          className="menu-pop menu-pop-top-left menu-pop-anim"
          role="dialog"
          aria-label={t('admin_nodes_filter')}
        >
          <div className="menu-head">
            <span className="text-sm font-bold text-(--theme-fg-default)">
              {t('admin_nodes_filter')}
            </span>
            <button
              type="button"
              onClick={() => setIsOpen(false)}
              className="icon-ghost"
              aria-label={t('common_close')}
              title={t('common_close')}
            >
              <X size={16} strokeWidth={2.5} />
            </button>
          </div>

          <div className="max-h-[60vh] overflow-y-auto p-2 custom-scrollbar">
            <div className="px-2 pb-1 pt-0.5 text-xs font-semibold uppercase tracking-wider text-(--theme-fg-muted)">
              {t('admin_nodes_filter_groups')}
            </div>
            {groups.length === 0 ? (
              <div className="px-3 py-4 text-sm text-(--theme-fg-muted)">
                {t('admin_nodes_filter_no_groups')}
              </div>
            ) : (
              groups.map((group) => {
                const selected = selectedGroupIds.includes(group.id);
                return (
                  <button
                    type="button"
                    role="option"
                    key={group.id}
                    aria-selected={selected}
                    onClick={() => toggleGroup(group.id)}
                    className={itemClass(selected)}
                  >
                    <span className={checkboxClass(selected)} aria-hidden="true">
                      {selected && <Check size={14} strokeWidth={3} />}
                    </span>
                    <span className="min-w-0 flex-1 truncate">{group.name}</span>
                    <span className="shrink-0 text-xs text-(--theme-fg-muted-alt)">
                      {group.server_count}
                    </span>
                  </button>
                );
              })
            )}

            {showVersionFilter && (
              <div className="mt-2 border-t border-(--theme-border-default) pt-2">
                <div className="px-2 pb-1 text-xs font-semibold uppercase tracking-wider text-(--theme-fg-muted)">
                  {t('admin_nodes_filter_version')}
                </div>
                <button
                  type="button"
                  role="option"
                  aria-selected={updateableOnly}
                  onClick={() => onUpdateableOnlyChange(!updateableOnly)}
                  className={itemClass(updateableOnly)}
                >
                  <span className={checkboxClass(updateableOnly)} aria-hidden="true">
                    {updateableOnly && <Check size={14} strokeWidth={3} />}
                  </span>
                  <span className="min-w-0 flex-1">
                    <span className="block truncate">{t('admin_nodes_filter_updateable')}</span>
                  </span>
                </button>
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  );
};

export default NodeFilterMenu;
