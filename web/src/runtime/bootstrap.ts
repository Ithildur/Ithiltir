import { ensureThemePackageRuntime } from '@lib/themePackageRuntime';
import { installAuthApiSession } from '@stores/authStore';
import { ensureThemeRuntime } from '@stores/themeModeBridge';
import { syncThemeRuntimeState } from '@stores/themeStore';

export const installBrowserRuntime = (): void => {
  installAuthApiSession();
  ensureThemeRuntime();
  ensureThemePackageRuntime();
  syncThemeRuntimeState();
};
