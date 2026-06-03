import RefreshCw from 'lucide-react/dist/esm/icons/refresh-cw';
import { useI18n } from '@i18n';

export const TrafficRebuildOverlay = () => {
  const { t } = useI18n();
  return (
    <div
      className="absolute inset-0 z-20 grid min-h-72 place-items-center overflow-hidden bg-(--theme-page-bg)/92 px-6 py-12 backdrop-blur-sm dark:bg-(--theme-bg-default)/92"
      aria-live="polite"
      role="status"
    >
      <div className="absolute inset-x-0 top-0 h-px bg-(--theme-border-subtle) dark:bg-(--theme-border-default)" />
      <div className="absolute inset-x-0 bottom-0 h-px bg-(--theme-border-subtle) dark:bg-(--theme-border-default)" />
      <div className="absolute inset-0 bg-[linear-gradient(135deg,var(--theme-fg-default)_1px,transparent_1px)] bg-size-[28px_28px] opacity-[0.06]" />

      <div className="relative flex w-full max-w-4xl flex-col items-center justify-center gap-8 text-center sm:flex-row sm:text-left">
        <div className="relative flex size-28 shrink-0 items-center justify-center sm:size-32">
          <div className="absolute inset-0 rounded-full border border-(--theme-border-subtle) bg-(--theme-bg-default)/80 shadow-xl dark:border-(--theme-border-default) dark:bg-(--theme-bg-inset)/80" />
          <div className="absolute inset-3 rounded-full border-2 border-(--theme-border-subtle) border-t-(--theme-fg-accent) animate-spin dark:border-(--theme-border-default) dark:border-t-(--theme-fg-accent)" />
          <RefreshCw className="relative size-10 text-(--theme-fg-accent)" aria-hidden="true" />
        </div>

        <div className="min-w-0">
          <div className="text-xs font-semibold uppercase tracking-[0.18em] text-(--theme-fg-accent)">
            {t('traffic_current_cycle')}
          </div>
          <div className="mt-3 max-w-2xl text-2xl/8 font-semibold tracking-tight text-(--theme-fg-default) sm:text-3xl/9">
            {t('traffic_rebuild_overlay_title')}
          </div>
          <div className="mt-3 max-w-xl text-sm/6 text-(--theme-fg-muted)">
            {t('traffic_rebuild_overlay_detail')}
          </div>
        </div>
      </div>
    </div>
  );
};
