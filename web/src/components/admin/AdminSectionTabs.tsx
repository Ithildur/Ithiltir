import type { LucideIcon } from 'lucide-react';
import { useI18n, type TranslationKey } from '@i18n';

interface AdminSectionTab<Key extends string> {
  key: Key;
  labelKey: TranslationKey;
  icon: LucideIcon;
}

interface Props<Key extends string> {
  tabs: readonly AdminSectionTab<Key>[];
  activeKey: Key;
  onChange: (key: Key) => void;
}

const tabClass = (active: boolean) =>
  `px-1 py-2 text-sm font-semibold border-b-2 -mb-px transition-colors ${
    active
      ? 'border-(--theme-border-underline-nav-active) text-(--theme-fg-default)'
      : 'border-transparent text-(--theme-fg-muted) dark:text-(--theme-fg-muted) hover:text-(--theme-fg-default) dark:hover:text-(--theme-fg-default)'
  }`;

export const AdminSectionTabs = <Key extends string>({ tabs, activeKey, onChange }: Props<Key>) => {
  const { t } = useI18n();

  return (
    <div className="flex gap-4 border-b border-(--theme-border-subtle) dark:border-(--theme-border-default)">
      {tabs.map((tab) => (
        <button
          key={tab.key}
          type="button"
          onClick={() => onChange(tab.key)}
          aria-current={activeKey === tab.key ? 'page' : undefined}
          className={tabClass(activeKey === tab.key)}
        >
          <span className="inline-flex items-center gap-2">
            <tab.icon className="size-4" aria-hidden="true" />
            {t(tab.labelKey)}
          </span>
        </button>
      ))}
    </div>
  );
};
