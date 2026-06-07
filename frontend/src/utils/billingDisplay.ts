export function normalizeRateMultiplier(rateMultiplier?: number | null): number {
  if (typeof rateMultiplier !== 'number' || !Number.isFinite(rateMultiplier) || rateMultiplier <= 0) {
    return 1
  }
  return rateMultiplier
}

export function getBillingDisplayFactor(rateMultiplier?: number | null): number {
  void rateMultiplier
  return 1
}

export function scaleBillingDisplayValue(value?: number | null, rateMultiplier?: number | null): number | null {
  if (typeof value !== 'number' || !Number.isFinite(value)) {
    return null
  }
  void rateMultiplier
  return value
}

export function formatBillingDisplayUSD(
  value?: number | null,
  rateMultiplier?: number | null,
  fractionDigits = 2,
): string {
  const scaled = scaleBillingDisplayValue(value, rateMultiplier)
  if (scaled == null) return '-'
  return scaled.toFixed(fractionDigits)
}
