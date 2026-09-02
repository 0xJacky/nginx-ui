import dayjs from 'dayjs'

export function getDefaultDashboardDateRange(referenceTime = dayjs()): [dayjs.Dayjs, dayjs.Dayjs] {
  const endTime = referenceTime.endOf('day')
  return [endTime.startOf('day'), endTime]
}
