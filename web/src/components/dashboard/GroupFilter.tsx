import React from 'react';
import { useI18n } from '@i18n';
import type { GroupView } from '@app-types/frontMetrics';
import MultiSelectFilter from '@components/ui/MultiSelectFilter';

interface Props {
  groups: GroupView[];
  selectedIds: number[];
  onChange: (ids: number[]) => void;
  customTrigger?: React.ReactNode;
  align?: 'left' | 'right';
  direction?: 'up' | 'down';
  variant?: 'default' | 'fab';
}

const GroupFilter: React.FC<Props> = ({
  groups,
  selectedIds,
  onChange,
  customTrigger,
  align = 'left',
  direction = 'down',
  variant = 'default',
}) => {
  const { t } = useI18n();
  const label = selectedIds.length === 0 ? t('all_groups') : t('admin_nodes_group_label');

  return (
    <MultiSelectFilter
      items={groups.map((group) => ({ id: group.id, label: group.name }))}
      selectedIds={selectedIds}
      onChange={onChange}
      label={label}
      title={t('admin_nodes_filter')}
      emptyLabel={t('no_data')}
      clearLabel={t('common_clear')}
      closeLabel={t('common_close')}
      customTrigger={customTrigger}
      align={align}
      direction={direction}
      variant={variant}
    />
  );
};

export default GroupFilter;
