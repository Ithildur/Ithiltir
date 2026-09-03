import { installAuthApiSession } from '@stores/authStore';
import { syncThemeRuntimeState } from '@stores/themeStore';

export const installBrowserRuntime = (): void => {
  installAuthApiSession();
  syncThemeRuntimeState();
};
