import type { UptimeHistory } from '@pages/dashboard/viewModel';

export const uptimeDays = 45;

type Chart = { width: number; history?: UptimeHistory };
const charts = new Map<HTMLCanvasElement, Chart>();
const pending = new Set<HTMLCanvasElement>();
let resizeObserver: ResizeObserver | null = null;
let themeObserver: MutationObserver | null = null;
let frame: number | null = null;

const draw = (canvas: HTMLCanvasElement, chart: Chart, colors: string[]) => {
  const { width, history } = chart;
  if (width <= 0) return;
  const context = canvas.getContext('2d');
  if (!context) return;
  const ratio = window.devicePixelRatio || 1;
  canvas.width = Math.round(width * ratio);
  canvas.height = Math.round(18 * ratio);
  context.setTransform(canvas.width / width, 0, 0, canvas.height / 18, 0, 0);
  const recent = history?.days.slice(-uptimeDays) ?? [];
  const step = (width + 1) / uptimeDays;
  for (let index = 0; index < uptimeDays; index += 1) {
    const percent = recent[index - (uptimeDays - recent.length)]?.percent;
    const color =
      percent == null || !history
        ? 0
        : percent < history.errorSLA
          ? 3
          : percent < history.warningSLA
            ? 2
            : 1;
    const left = Math.round(index * step * ratio) / ratio;
    const right = Math.round(((index + 1) * step - 1) * ratio) / ratio;
    const barWidth = Math.max(0, right - left);
    context.fillStyle = colors[color];
    context.beginPath();
    context.roundRect(left, 2, barWidth, 14, Math.min(2, barWidth / 2));
    context.fill();
  }
};

const schedule = (canvas: HTMLCanvasElement) => {
  pending.add(canvas);
  if (frame !== null) return;
  frame = requestAnimationFrame(() => {
    frame = null;
    const style = getComputedStyle(document.documentElement);
    const colors = [
      '--theme-border-default',
      '--theme-fg-success-muted',
      '--theme-fg-warning-muted',
      '--theme-fg-danger-muted',
    ].map((token) => style.getPropertyValue(token).trim());
    for (const target of pending) {
      const chart = charts.get(target);
      if (chart) draw(target, chart, colors);
    }
    pending.clear();
  });
};

const redraw = () => charts.forEach((_, canvas) => schedule(canvas));
const stylesheetLoaded = (event: Event) => {
  if (event.target instanceof HTMLLinkElement && event.target.rel === 'stylesheet') redraw();
};

// All mounted rows share observers; metric polling and pointer movement do not draw.
export const observeUptimeCanvas = (canvas: HTMLCanvasElement, history?: UptimeHistory) => {
  if (charts.size === 0) {
    resizeObserver = new ResizeObserver((entries) => {
      for (const entry of entries) {
        const target = entry.target as HTMLCanvasElement;
        const chart = charts.get(target);
        if (chart && chart.width !== entry.contentRect.width) {
          chart.width = entry.contentRect.width;
          schedule(target);
        }
      }
    });
    themeObserver = new MutationObserver(redraw);
    themeObserver.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ['class', 'style'],
    });
    document.addEventListener('load', stylesheetLoaded, true);
    window.addEventListener('resize', redraw);
  }
  charts.set(canvas, { width: 0, history });
  resizeObserver?.observe(canvas);
  return () => {
    resizeObserver?.unobserve(canvas);
    charts.delete(canvas);
    pending.delete(canvas);
    if (charts.size !== 0) return;
    resizeObserver?.disconnect();
    themeObserver?.disconnect();
    resizeObserver = null;
    themeObserver = null;
    document.removeEventListener('load', stylesheetLoaded, true);
    window.removeEventListener('resize', redraw);
    if (frame !== null) cancelAnimationFrame(frame);
    frame = null;
  };
};
