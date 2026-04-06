/**
 * ESM default export matching throughput@1.0.2 (MIT, ThaUnknown).
 * The published package is CJS-only (`module.exports`); jassub's debug.js uses
 * `import throughput from "throughput"`, which breaks in the browser when Vite
 * serves the raw file. Alias this module as `throughput` in vite.config.ts.
 *
 * Behavior matches the package's non-hrtime (browser) branch only — sufficient for JASSUB.
 */
const maxTick = 65535;
const resolution = 10;
const timeDiff = 1e3 / resolution;

const now = () => performance.now();

function getTick(start: number): number {
  return ((now() - start) / timeDiff) & maxTick;
}

export default function throughput(seconds?: number): (delta?: number) => number {
  const start = now();
  const size = resolution * (seconds || 5);
  const buffer: number[] = [0];
  let pointer = 1;
  let last = (getTick(start) - 1) & maxTick;

  return function (delta?: number): number {
    const tick = getTick(start);
    let dist = (tick - last) & maxTick;
    if (dist > size) dist = size;
    last = tick;

    while (dist--) {
      if (pointer === size) pointer = 0;
      buffer[pointer] = buffer[pointer === 0 ? size - 1 : pointer - 1]!;
      pointer++;
    }

    if (delta) buffer[pointer - 1]! += delta;

    const top = buffer[pointer - 1]!;
    const btm = buffer.length < size ? 0 : buffer[pointer === size ? 0 : pointer]!;

    return buffer.length < resolution ? top : (top - btm) * resolution / buffer.length;
  };
}
