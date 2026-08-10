import { useI18n } from './i18n'

export function useAccessCopy() {
  const { locale } = useI18n()
  return {
    locale,
    get value() {
      return locale.value === 'zh' ? {
        userManagement: '\u7528\u6237\u7ba1\u7406', roleManagement: '\u6743\u9650\u4e0e\u89d2\u8272',
        userManagementHint: '\u521b\u5efa\u3001\u7981\u7528\u6216\u91cd\u7f6e\u540e\u53f0\u7528\u6237\u8d26\u53f7\uff1b\u7528\u6237\u7ba1\u7406\u4ec5\u9650 admin \u89d2\u8272\u3002',
        roleManagementHint: '\u914d\u7f6e\u89d2\u8272\u53ef\u8bbf\u95ee\u7684\u540e\u53f0\u529f\u80fd\uff1badmin \u89d2\u8272\u59cb\u7ec8\u62e5\u6709\u5168\u90e8\u6743\u9650\u3002',
        addUser: '\u65b0\u589e\u7528\u6237', editUser: '\u7f16\u8f91\u7528\u6237', addRole: '\u65b0\u589e\u89d2\u8272', editRole: '\u7f16\u8f91\u89d2\u8272',
        noRole: '\u672a\u5206\u914d', noPermissions: '\u65e0\u6743\u9650', allPermissions: '\u5168\u90e8\u6743\u9650', passwordKeepHint: '\u7559\u7a7a\u4ee5\u4fdd\u7559\u5f53\u524d\u5bc6\u7801',
        userSaveError: '\u7528\u6237\u4fdd\u5b58\u5931\u8d25', roleSaveError: '\u89d2\u8272\u4fdd\u5b58\u5931\u8d25',
      } : {
        userManagement: 'User management', roleManagement: 'Roles and permissions',
        userManagementHint: 'Create, disable, or reset console accounts. Only the admin role can manage users.',
        roleManagementHint: 'Choose the console permissions for each role. The admin role always has full access.',
        addUser: 'Add user', editUser: 'Edit user', addRole: 'Add role', editRole: 'Edit role',
        noRole: 'Unassigned', noPermissions: 'No permissions', allPermissions: 'All permissions', passwordKeepHint: 'Leave blank to keep the current password',
        userSaveError: 'Unable to save user', roleSaveError: 'Unable to save role',
      }
    },
  }
}
