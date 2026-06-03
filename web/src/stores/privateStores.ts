import { resetAdminGroupsStore } from './adminGroupsStore';
import { resetAdminNodesStore } from './adminNodesStore';
import { resetAlertChannelsStore } from './alertChannelsStore';
import { resetAlertMountsStore } from './alertMountsStore';
import { resetAlertRulesStore } from './alertRulesStore';
import { resetStatisticsAccessStore } from './statisticsAccessStore';
import { resetTrafficRebuild } from './trafficRebuildStore';
import { resetTrafficSettingsStore } from './trafficSettingsStore';

export const resetPrivateStores = (): void => {
  resetAdminGroupsStore();
  resetAdminNodesStore();
  resetAlertChannelsStore();
  resetAlertMountsStore();
  resetAlertRulesStore();
  resetStatisticsAccessStore();
  resetTrafficRebuild();
  resetTrafficSettingsStore();
};
