import React from 'react';
import Search from 'lucide-react/dist/esm/icons/search';
import Upload from 'lucide-react/dist/esm/icons/upload';
import Button from '@components/ui/Button';
import Card from '@components/ui/Card';
import Input from '@components/ui/Input';
import { useI18n } from '@i18n';
import type { ConfirmAction } from '@hooks/useConfirmDialog';
import { useThemePackages } from '@components/admin/systemManager/hooks/useThemePackages';
import { DefaultThemeRow, ThemeRow } from '@components/admin/systemManager/ThemeRow';
import {
  matchesThemeSearch,
  normalizeThemeSearch,
  type DefaultThemeOption,
} from '@components/admin/systemManager/themeManagerModel';
import { defaultThemeManifest } from '@lib/themePackageRuntime';

const ThemeManager: React.FC<{
  enabled: boolean;
  confirmAction: ConfirmAction;
}> = ({ enabled, confirmAction }) => {
  const { t } = useI18n();
  const inputRef = React.useRef<HTMLInputElement | null>(null);
  const [search, setSearch] = React.useState('');
  const {
    packages,
    loading,
    isBusy,
    uploading,
    applyingId,
    defaultActive,
    uploadTheme,
    applyTheme,
    deleteTheme,
  } = useThemePackages({
    enabled,
    confirmAction,
  });
  const keyword = React.useMemo(() => normalizeThemeSearch(search), [search]);
  const defaultOption = React.useMemo<DefaultThemeOption>(
    () => ({
      ...defaultThemeManifest,
      active: defaultActive,
    }),
    [defaultActive],
  );
  const showDefault = React.useMemo(
    () => matchesThemeSearch(defaultOption, keyword),
    [defaultOption, keyword],
  );

  const filteredPackages = React.useMemo(() => {
    return packages.filter((item) => matchesThemeSearch(item, keyword));
  }, [packages, keyword]);

  const onPickFile = React.useCallback(() => {
    inputRef.current?.click();
  }, []);

  const onFileChange = React.useCallback(
    async (event: React.ChangeEvent<HTMLInputElement>) => {
      const file = event.target.files?.[0];
      if (!file) return;
      await uploadTheme(file);
      event.target.value = '';
    },
    [uploadTheme],
  );

  return (
    <div className="space-y-4 md:space-y-6">
      <input
        ref={inputRef}
        type="file"
        accept=".zip,application/zip"
        className="hidden"
        onChange={onFileChange}
      />

      <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
        <Input
          icon={Search}
          placeholder={t('admin_theme_search_placeholder')}
          aria-label={t('admin_theme_search_placeholder')}
          value={search}
          onChange={(event) => setSearch(event.target.value)}
          data-search-input="true"
          wrapperClassName="w-full lg:max-w-md"
        />

        <div className="flex w-full shrink-0 items-center gap-2 lg:w-auto">
          <Button icon={Upload} className="w-full lg:w-auto" onClick={onPickFile} disabled={isBusy}>
            {uploading ? t('admin_theme_uploading') : t('admin_theme_upload')}
          </Button>
        </div>
      </div>

      <Card className="overflow-hidden">
        <div className="overflow-x-auto">
          <table className="w-full bg-(--theme-bg-default) text-left text-sm dark:bg-(--theme-bg-default)">
            <thead className="border-b border-(--theme-border-subtle) bg-(--theme-bg-muted) text-xs font-semibold text-(--theme-fg-default) dark:border-(--theme-border-default) dark:bg-(--theme-canvas-subtle)">
              <tr>
                <th className="px-4 py-3">{t('admin_theme_col_preview')}</th>
                <th className="px-4 py-3">{t('admin_theme_col_skin')}</th>
                <th className="px-4 py-3">{t('admin_theme_col_meta')}</th>
                <th className="px-4 py-3">{t('admin_theme_col_updated')}</th>
                <th className="px-4 py-3">{t('admin_theme_col_capabilities')}</th>
                <th className="px-4 py-3 text-right">{t('common_actions')}</th>
              </tr>
            </thead>

            <tbody className="divide-y divide-(--theme-border-muted) dark:divide-(--theme-canvas-muted)">
              {showDefault && (
                <DefaultThemeRow
                  item={defaultOption}
                  busy={isBusy}
                  applying={applyingId === defaultOption.id}
                  onApply={() => void applyTheme(defaultOption)}
                />
              )}

              {loading && filteredPackages.length === 0 ? (
                <tr>
                  <td
                    colSpan={6}
                    className="px-4 py-12 text-center text-(--theme-fg-muted) dark:text-(--theme-fg-neutral)"
                  >
                    {t('admin_theme_loading')}
                  </td>
                </tr>
              ) : filteredPackages.length === 0 && !showDefault ? (
                <tr>
                  <td
                    colSpan={6}
                    className="px-4 py-12 text-center text-(--theme-fg-muted) dark:text-(--theme-fg-neutral)"
                  >
                    {t('admin_theme_empty')}
                  </td>
                </tr>
              ) : (
                filteredPackages.map((item) => (
                  <ThemeRow
                    key={item.id}
                    item={item}
                    busy={isBusy}
                    applying={applyingId === item.id}
                    onApply={() => void applyTheme(item)}
                    onDelete={() => void deleteTheme(item)}
                  />
                ))
              )}
            </tbody>
          </table>
        </div>
      </Card>
    </div>
  );
};

export default ThemeManager;
