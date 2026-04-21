import fs from "node:fs/promises";
import path from "node:path";
import os from "node:os";
import { fileURLToPath } from "node:url";
import { spawn } from "node:child_process";

const scriptDir = path.dirname(fileURLToPath(import.meta.url));
const mobileDir = path.resolve(scriptDir, "..");
const rootDir = path.resolve(mobileDir, "..");
const outputDir = path.resolve(mobileDir, "store-assets", "google-play");
const serverExe = path.resolve(rootDir, "cellgame.exe");
const edgeExe = process.env.EDGE_PATH || "C:\\Program Files (x86)\\Microsoft\\Edge\\Application\\msedge.exe";
const serverUrl = process.env.CELLGAME_CAPTURE_URL || "http://127.0.0.1:8000";
const debugPort = 9222;
const viewport = { width: 1920, height: 1080 };
const mobileUserAgent = [
  "Mozilla/5.0 (Linux; Android 16; SM-F966N)",
  "AppleWebKit/537.36 (KHTML, like Gecko)",
  "Chrome/135.0.0.0 Mobile Safari/537.36",
  "CellGameAndroidWebView",
].join(" ");

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

async function exists(targetPath) {
  try {
    await fs.access(targetPath);
    return true;
  } catch {
    return false;
  }
}

async function waitForHttpOk(url, timeoutMs) {
  const startedAt = Date.now();
  while (Date.now() - startedAt < timeoutMs) {
    try {
      const response = await fetch(url, { redirect: "manual" });
      if (response.ok) {
        return;
      }
    } catch {
      // Ignore boot-time connection errors and keep polling.
    }
    await sleep(250);
  }
  throw new Error(`Timed out waiting for ${url}`);
}

function spawnHidden(command, args, options = {}) {
  const child = spawn(command, args, {
    ...options,
    stdio: "ignore",
    windowsHide: true,
  });
  child.unref();
  return child;
}

async function terminateProcess(child) {
  if (!child || child.killed) {
    return;
  }
  const closed = new Promise((resolve) => {
    child.once("exit", resolve);
    child.once("close", resolve);
  });
  child.kill("SIGTERM");
  await Promise.race([closed, sleep(1500)]);
}

class CdpClient {
  constructor(socket) {
    this.socket = socket;
    this.nextId = 1;
    this.pending = new Map();
    this.eventWaiters = [];
    this.openPromise = new Promise((resolve, reject) => {
      socket.addEventListener("open", resolve, { once: true });
      socket.addEventListener("error", reject, { once: true });
    });

    socket.addEventListener("message", (event) => {
      const message = JSON.parse(event.data);
      if (message.id) {
        const pending = this.pending.get(message.id);
        if (!pending) {
          return;
        }
        this.pending.delete(message.id);
        if (message.error) {
          pending.reject(new Error(message.error.message || "CDP error"));
          return;
        }
        pending.resolve(message.result || {});
        return;
      }

      if (!message.method) {
        return;
      }

      for (const waiter of [...this.eventWaiters]) {
        if (waiter.method !== message.method) {
          continue;
        }
        if (!waiter.predicate(message.params || {})) {
          continue;
        }
        this.eventWaiters = this.eventWaiters.filter((item) => item !== waiter);
        waiter.resolve(message.params || {});
      }
    });

    socket.addEventListener("close", () => {
      for (const pending of this.pending.values()) {
        pending.reject(new Error("CDP socket closed"));
      }
      this.pending.clear();
    });
  }

  async ready() {
    await this.openPromise;
  }

  async send(method, params = {}) {
    const id = this.nextId++;
    const payload = JSON.stringify({ id, method, params });
    const response = new Promise((resolve, reject) => {
      this.pending.set(id, { resolve, reject });
    });
    this.socket.send(payload);
    return response;
  }

  waitForEvent(method, predicate = () => true, timeoutMs = 10000) {
    return new Promise((resolve, reject) => {
      const timeout = setTimeout(() => {
        this.eventWaiters = this.eventWaiters.filter((item) => item !== waiter);
        reject(new Error(`Timed out waiting for ${method}`));
      }, timeoutMs);

      const waiter = {
        method,
        predicate,
        resolve: (params) => {
          clearTimeout(timeout);
          resolve(params);
        },
      };
      this.eventWaiters.push(waiter);
    });
  }

  async evaluate(expression) {
    const result = await this.send("Runtime.evaluate", {
      expression,
      awaitPromise: true,
      returnByValue: true,
    });
    if (result.exceptionDetails) {
      const description = result.exceptionDetails.text || "Runtime.evaluate failed";
      throw new Error(description);
    }
    return result.result?.value;
  }

  close() {
    this.socket.close();
  }
}

async function connectToPageTarget() {
  const startedAt = Date.now();
  while (Date.now() - startedAt < 10000) {
    try {
      const response = await fetch(`http://127.0.0.1:${debugPort}/json/list`);
      const targets = await response.json();
      const page = targets.find((item) => item.type === "page" && item.webSocketDebuggerUrl);
      if (page) {
        const socket = new WebSocket(page.webSocketDebuggerUrl);
        const client = new CdpClient(socket);
        await client.ready();
        return client;
      }
    } catch {
      // Keep polling while Edge starts.
    }
    await sleep(250);
  }
  throw new Error("Timed out waiting for Edge remote debugging target");
}

async function configureMobilePage(client) {
  await client.send("Page.enable");
  await client.send("Runtime.enable");
  await client.send("Network.enable");
  await client.send("Emulation.setDeviceMetricsOverride", {
    width: viewport.width,
    height: viewport.height,
    deviceScaleFactor: 1,
    mobile: true,
    screenWidth: viewport.width,
    screenHeight: viewport.height,
    positionX: 0,
    positionY: 0,
    scale: 1,
    screenOrientation: {
      type: "landscapePrimary",
      angle: 90,
    },
  });
  await client.send("Emulation.setTouchEmulationEnabled", {
    enabled: true,
    maxTouchPoints: 5,
  });
  await client.send("Network.setUserAgentOverride", {
    userAgent: mobileUserAgent,
    acceptLanguage: "ko-KR,ko,en-US,en",
    platform: "Android",
  });
}

async function waitForExpression(client, expression, description, timeoutMs = 15000) {
  const startedAt = Date.now();
  while (Date.now() - startedAt < timeoutMs) {
    const result = await client.evaluate(expression);
    if (result) {
      return result;
    }
    await sleep(200);
  }
  throw new Error(`Timed out waiting for ${description}`);
}

async function navigate(client, url) {
  const loadEvent = client.waitForEvent("Page.loadEventFired", () => true, 15000);
  await client.send("Page.navigate", { url });
  await loadEvent;
  await waitForExpression(client, "document.readyState === 'complete'", "document ready state");
}

async function capture(client, filename) {
  const { data } = await client.send("Page.captureScreenshot", {
    format: "png",
    fromSurface: true,
    captureBeyondViewport: false,
  });
  await fs.writeFile(path.join(outputDir, filename), Buffer.from(data, "base64"));
}

async function startSession(client, { nickname, cellType }) {
  await navigate(client, serverUrl);
  await waitForExpression(
    client,
    "!!document.querySelector('.cell-option[data-cell-type=\"classic\"]')",
    "login screen",
  );
  await client.evaluate(`
    (() => {
      document.querySelector('.cell-option[data-cell-type="${cellType}"]')?.click();
      const nicknameInput = document.getElementById('nicknameInput');
      nicknameInput.value = ${JSON.stringify(nickname)};
      nicknameInput.dispatchEvent(new Event('input', { bubbles: true }));
      document.querySelector('#loginForm button[type="submit"]')?.click();
      return true;
    })()
  `);
  await waitForExpression(
    client,
    "document.getElementById('hud') && !document.getElementById('hud').classList.contains('hidden')",
    "game HUD",
    20000,
  );
  await sleep(2200);
  await client.evaluate(`
    (() => {
      const button = document.getElementById('leaderboardToggle');
      if (button && button.textContent.includes('열기')) {
        button.click();
      }
      return true;
    })()
  `);
  await sleep(500);
}

async function releaseKeys(client) {
  await client.evaluate(`
    (() => {
      window.dispatchEvent(new KeyboardEvent('keyup', { key: ' ', code: 'Space', bubbles: true }));
      window.dispatchEvent(new KeyboardEvent('keyup', { key: 'w', code: 'KeyW', bubbles: true }));
      return true;
    })()
  `);
}

async function pressKey(client, key, code) {
  await client.evaluate(`
    (() => {
      window.dispatchEvent(new KeyboardEvent('keydown', { key: ${JSON.stringify(key)}, code: ${JSON.stringify(code)}, bubbles: true }));
      return true;
    })()
  `);
}

async function captureGameplayShots(client) {
  await startSession(client, { nickname: "StorePlay1", cellType: "classic" });
  await capture(client, "phone-screenshot-01-classic-gameplay.png");

  await startSession(client, { nickname: "StorePlay2", cellType: "giant" });
  await pressKey(client, " ", "Space");
  await sleep(650);
  await capture(client, "phone-screenshot-02-giant-ability.png");
  await releaseKeys(client);

  await startSession(client, { nickname: "StorePlay3", cellType: "divider" });
  await pressKey(client, "w", "KeyW");
  await sleep(900);
  await capture(client, "phone-screenshot-03-divider-split.png");
  await releaseKeys(client);
}

async function captureLoginShot(client) {
  await navigate(client, serverUrl);
  await waitForExpression(
    client,
    "!!document.querySelector('.cell-option[data-cell-type=\"classic\"]')",
    "login screen",
  );
  await client.evaluate(`
    (() => {
      document.querySelector('.cell-option[data-cell-type="giant"]')?.click();
      return true;
    })()
  `);
  await sleep(500);
  await capture(client, "phone-screenshot-04-login-selection.png");
}

async function main() {
  if (!(await exists(serverExe))) {
    throw new Error(`Server executable not found: ${serverExe}`);
  }
  if (!(await exists(edgeExe))) {
    throw new Error(`Edge executable not found: ${edgeExe}`);
  }

  await fs.mkdir(outputDir, { recursive: true });

  const profileDir = await fs.mkdtemp(path.join(os.tmpdir(), "cellgame-edge-profile-"));
  const server = spawnHidden(serverExe, [], { cwd: rootDir });
  let edge;
  let client;

  try {
    await waitForHttpOk(serverUrl, 15000);

    edge = spawnHidden(edgeExe, [
      "--headless=new",
      "--disable-gpu",
      "--hide-scrollbars",
      `--remote-debugging-port=${debugPort}`,
      `--user-data-dir=${profileDir}`,
      "about:blank",
    ]);

    client = await connectToPageTarget();
    await configureMobilePage(client);
    await captureGameplayShots(client);
    await captureLoginShot(client);
  } finally {
    if (client) {
      client.close();
    }
    await terminateProcess(edge);
    await terminateProcess(server);
    try {
      await fs.rm(profileDir, { recursive: true, force: true, maxRetries: 5, retryDelay: 250 });
    } catch {
      // Ignore profile cleanup failures; they do not affect the generated assets.
    }
  }
}

main().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
