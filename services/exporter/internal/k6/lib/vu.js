// Shared VU allocation helper. Formula: preAllocatedVUs = ceil(target_rps * 1.5). maxVUs = 2 × pre.

export const VU_RATIO = 1.5;
export const MAX_RATIO = 2;

export function preAllocatedVUs(targetRps) {
  if (!targetRps || targetRps <= 0) return 1;
  return Math.ceil(targetRps * VU_RATIO);
}

export function maxVUs(targetRps) {
  return preAllocatedVUs(targetRps) * MAX_RATIO;
}
