import dayjs from 'dayjs'

export interface StructuredTimeRange {
  start: dayjs.Dayjs
  end: dayjs.Dayjs
}

export function getInitialStructuredTimeRange(
  indexedStart?: dayjs.Dayjs,
  indexedEnd = dayjs(),
): StructuredTimeRange {
  const windowStart = indexedEnd.subtract(24, 'hour')
  const boundedStart = indexedStart?.isAfter(windowStart) ? indexedStart : windowStart

  return {
    start: boundedStart.isAfter(indexedEnd) ? indexedEnd : boundedStart,
    end: indexedEnd,
  }
}
