import type { AlertSemanticType } from 'antdv-next'

/** Tighter spacing for the alerts with a description in the install dialog. */
export const compactAlertStyles: AlertSemanticType['styles'] = {
  root: { padding: '12px 16px', alignItems: 'flex-start' },
  icon: { fontSize: '16px', marginTop: '3px', marginInlineEnd: '10px' },
  title: { fontSize: '14px', fontWeight: 500, marginBottom: '2px' },
  description: { fontSize: '13px', lineHeight: '20px' },
}
