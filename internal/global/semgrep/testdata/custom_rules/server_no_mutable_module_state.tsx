// Violation 1: module-level let variable
let activeConnections = 0;

// Violation 2: module-level var variable
var globalConfig = {};

// Safe 1: module-level const variable
const MAX_LIMIT = 100;

export function handleRequest() {
  // Safe 2: local variable inside function
  let requestCount = 0;
  var temp = "ok";
  requestCount++;
  console.log(requestCount, temp, activeConnections, globalConfig, MAX_LIMIT);
}

export class SafeController {
  // Safe 3: class field
  activeUser = null;
  
  process() {
    // Safe 4: local variable inside method
    let x = 1;
    console.log(x, this.activeUser);
  }
}
