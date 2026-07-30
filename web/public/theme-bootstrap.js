/* global document, window */

(function () {
  // Prevent theme flicker by applying theme before React loads.
  var THEME_COOKIE_KEY = 'theme';
  var VALID = { light: true, dark: true, system: true };
  var root = document.documentElement;
  var LIGHT_THEME_COLOR = '#f8fafc';
  var DARK_THEME_COLOR = '#0d1117';
  var media =
    typeof window.matchMedia === 'function'
      ? window.matchMedia('(prefers-color-scheme: dark)')
      : null;

  function setThemeColor(isDark) {
    var meta = document.querySelector('meta[name="theme-color"]');
    if (!meta) {
      meta = document.createElement('meta');
      meta.setAttribute('name', 'theme-color');
      document.head.appendChild(meta);
    }
    meta.setAttribute('content', isDark ? DARK_THEME_COLOR : LIGHT_THEME_COLOR);
  }

  function getCookieTheme() {
    var match = document.cookie.match(/(?:^|; )theme=([^;]+)/);
    if (!match) return 'system';
    try {
      var value = decodeURIComponent(match[1]);
      return VALID[value] ? value : 'system';
    } catch {
      return 'system';
    }
  }

  function setCookieTheme(theme) {
    var oneYear = 60 * 60 * 24 * 365;
    document.cookie =
      THEME_COOKIE_KEY +
      '=' +
      encodeURIComponent(theme) +
      '; Max-Age=' +
      oneYear +
      '; Path=/; SameSite=Lax';
  }

  function applyThemeToDom(mode) {
    var isDark = mode === 'dark' || (mode === 'system' && media && media.matches);
    if (isDark) root.classList.add('dark');
    else root.classList.remove('dark');
    root.style.colorScheme = isDark ? 'dark' : 'light';
    setThemeColor(isDark);
  }

  function onSystemThemeChange(handler) {
    if (!media || !media.addEventListener) return function () {};
    media.addEventListener('change', handler);
    return function () {
      media.removeEventListener('change', handler);
    };
  }

  function loadActiveThemeManifest() {
    if (!window.fetch) return Promise.resolve(null);
    return window
      .fetch('/theme/active.json', {
        cache: 'no-store',
        credentials: 'same-origin',
      })
      .then(function (response) {
        if (!response.ok) return null;
        return response.json();
      })
      .catch(function () {
        return null;
      });
  }

  window.__theme = {
    get: getCookieTheme,
    set: setCookieTheme,
    apply: applyThemeToDom,
    onSystemChange: onSystemThemeChange,
  };

  var themePackage = {
    manifest: null,
    manifestPromise: loadActiveThemeManifest().then(function (manifest) {
      themePackage.manifest = manifest;
      return manifest;
    }),
    refresh: function () {
      var link = document.getElementById('active-theme-css');
      if (!link) return Promise.reject(new Error('active theme stylesheet is missing'));

      return new Promise(function (resolve, reject) {
        var timeout = window.setTimeout(function () {
          cleanup();
          reject(new Error('active theme stylesheet load timed out'));
        }, 15000);

        function cleanup() {
          window.clearTimeout(timeout);
          link.removeEventListener('load', onLoad);
          link.removeEventListener('error', onError);
        }
        function onLoad() {
          cleanup();
          resolve();
        }
        function onError() {
          cleanup();
          reject(new Error('failed to load active theme stylesheet'));
        }

        link.addEventListener('load', onLoad);
        link.addEventListener('error', onError);
        link.setAttribute('href', '/theme/active.css?v=' + Date.now());
      });
    },
  };
  window.__themePackage = themePackage;

  applyThemeToDom(getCookieTheme());
})();
