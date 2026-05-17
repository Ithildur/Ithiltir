import type { TranslationKey } from '@i18n';

type T = (key: TranslationKey, vars?: Record<string, string | number>) => string;

interface NamedRule {
  name: string;
  builtin?: boolean;
}

const builtinRuleKeys: Record<string, TranslationKey> = {
  node_offline: 'admin_alerts_mounts_rule_offline',
  raid_failed: 'admin_alerts_mounts_rule_raid',
  smart_failed: 'admin_alerts_mounts_rule_smart_failed',
  smart_nvme_critical_warning: 'admin_alerts_mounts_rule_smart_nvme_critical',
};

const metricKeys: Record<string, TranslationKey> = {
  'node.offline': 'admin_alerts_metric_node_offline',
  'raid.failed': 'admin_alerts_metric_raid_failed',
  'cpu.usage_ratio': 'admin_alerts_metric_cpu_usage_ratio',
  'cpu.load1': 'admin_alerts_metric_cpu_load1',
  'cpu.load5': 'admin_alerts_metric_cpu_load5',
  'cpu.load15': 'admin_alerts_metric_cpu_load15',
  'mem.used': 'admin_alerts_metric_mem_used',
  'mem.used_ratio': 'admin_alerts_metric_mem_used_ratio',
  'disk.usage.used_ratio': 'admin_alerts_metric_disk_usage_used_ratio',
  'disk.smart.failed': 'admin_alerts_metric_smart_failed',
  'disk.smart.nvme.critical_warning': 'admin_alerts_metric_smart_nvme_critical',
  'disk.smart.attribute_failing': 'admin_alerts_metric_smart_attribute_failing',
  'disk.smart.max_temp_c': 'admin_alerts_metric_smart_max_temp_c',
  'net.recv_bps': 'admin_alerts_metric_net_recv_bps',
  'net.sent_bps': 'admin_alerts_metric_net_sent_bps',
  'conn.tcp': 'admin_alerts_metric_conn_tcp',
  'thermal.max_temp_c': 'admin_alerts_metric_thermal_max_temp_c',
};

export const alertMetricValues = Object.keys(metricKeys);

export const alertRuleName = (rule: NamedRule, t: T): string => {
  if (!rule.builtin) return rule.name;
  const key = builtinRuleKeys[rule.name];
  return key ? t(key) : rule.name;
};

export const alertMetricName = (metric: string, t: T): string => {
  const key = metricKeys[metric];
  return key ? t(key) : metric;
};
