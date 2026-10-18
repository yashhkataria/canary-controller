import http from 'k6/http';

const RATE = parseInt(__ENV.RATE || '50', 10);
const DURATION = __ENV.DURATION || '10m';
const TARGET = __ENV.TARGET || 'http://demo.default.svc.cluster.local';

export const options = {
  noConnectionReuse: true,
  discardResponseBodies: true,
  scenarios: {
    steady: {
      executor: 'constant-arrival-rate',
      rate: RATE,
      timeUnit: '1s',
      duration: DURATION,
      preAllocatedVUs: Math.max(10, RATE),
      maxVUs: Math.max(50, RATE * 4),
    },
  },
};

export default function () {
  http.get(`${TARGET}/`, { timeout: '5s' });
}
