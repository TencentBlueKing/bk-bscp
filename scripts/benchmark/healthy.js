import http from "k6/http";
import { check, sleep } from "k6";

// Test configuration
export const options = {
  vus: 100,
  duration: "30s",
};

// Simulated user behavior
export default function () {
  let res = http.get("http://127.0.0.1:8080/-/healthy");

  // Validate response status
  check(res, { "status was 200": (r) => r.status == 200 });

  sleep(Math.random() * 0.1);
}
