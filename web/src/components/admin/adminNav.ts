import type { LucideIcon } from 'lucide-react';
import type { AdminConsoleTab } from '@app-types/admin';
import type { TranslationKey } from '@i18n';

export interface AdminNavItem {
  key: AdminConsoleTab;
  labelKey: TranslationKey;
  icon: LucideIcon;
  hidden?: boolean;
}
