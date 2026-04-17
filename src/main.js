const canvas = document.getElementById("gameCanvas");
const ctx = canvas.getContext("2d");

const loginScreen = document.getElementById("loginScreen");
const loginForm = document.getElementById("loginForm");
const nicknameInput = document.getElementById("nicknameInput");
const cellOptions = document.getElementById("cellOptions");
const hud = document.getElementById("hud");
const leaderboard = document.getElementById("leaderboard");
const minimap = document.getElementById("minimap");
const minimapCanvas = document.getElementById("minimapCanvas");
const minimapCtx = minimapCanvas.getContext("2d");
const messageBox = document.getElementById("message");
const hudName = document.getElementById("hudName");
const hudCellType = document.getElementById("hudCellType");
const hudMass = document.getElementById("hudMass");
const hudRank = document.getElementById("hudRank");
const hudAbilityName = document.getElementById("hudAbilityName");
const hudCooldownLabel = document.getElementById("hudCooldownLabel");
const hudCooldownFill = document.getElementById("hudCooldownFill");
const hudEffectLabel = document.getElementById("hudEffectLabel");
const hudEffectFill = document.getElementById("hudEffectFill");

const CELL_TYPES = {
  classic: {
    name: "기본 세포",
    abilityName: "질주",
    description: "짧은 시간 빠르게 치고 빠집니다. 가장 다루기 쉬운 기본형입니다.",
    detail: "4초 쿨타임 / 1.2초 가속",
    cooldownMs: 4000,
    effectMs: 1200,
  },
  blink: {
    name: "블링크 세포",
    abilityName: "순간이동",
    description: "마우스 방향으로 짧게 순간이동합니다.",
    detail: "6초 쿨타임",
    cooldownMs: 6000,
    effectMs: 0,
  },
  giant: {
    name: "자이언트 세포",
    abilityName: "거대화",
    description: "잠시 반경을 2배 가까이 키우고 느려집니다. 커진 만큼 방어가 강화되지만 공격은 불가합니다.",
    detail: "10초 쿨타임 / 5초 지속 / 공격 불가",
    cooldownMs: 10000,
    effectMs: 5000,
  },
  shield: {
    name: "실드 세포",
    abilityName: "보호막",
    description: "짧은 시간 포식당하지 않는 방어막을 전개합니다.",
    detail: "12초 쿨타임 / 3초 무적",
    cooldownMs: 12000,
    effectMs: 3000,
  },
  magnet: {
    name: "마그넷 세포",
    abilityName: "흡착",
    description: "주변 먹이를 끌어당겨 성장 루트를 빠르게 확보합니다.",
    detail: "9초 쿨타임 / 4초 흡착",
    cooldownMs: 9000,
    effectMs: 4000,
  },
};

const world = {
  width: 3600,
  height: 3600,
};

const MINIMAP_RANGE = 900;

const state = {
  playerId: null,
  sessionId: null,
  nickname: "",
  players: [],
  renderPlayers: new Map(),
  foods: [],
  cacti: [],
  mouse: { x: window.innerWidth / 2, y: window.innerHeight / 2 },
  camera: { x: 0, y: 0 },
  lastFrame: 0,
  connected: false,
  messageTimer: 0,
  pendingDirection: { x: 0, y: 0 },
  socket: null,
  inputTimer: null,
  selectedCellType: "classic",
  abilityPressed: false,
  splitPressed: false,
  zoom: 1,
  zoomTarget: 1,
  zoomReturnAt: 0,
  reconnectTimer: null,
  reconnectAttempts: 0,
  lastSnapshotAt: 0,
  time: 0,
};

window.addEventListener("resize", resizeCanvas);
window.addEventListener("keydown", (event) => {
  if (event.code === "Space" && !event.repeat) {
    state.abilityPressed = true;
  }
  if (event.code === "KeyW" && !event.repeat) {
    state.splitPressed = true;
  }
});
canvas.addEventListener("mousemove", (event) => {
  state.mouse.x = event.clientX;
  state.mouse.y = event.clientY;
});
canvas.addEventListener("wheel", (event) => {
  event.preventDefault();
  const delta = event.deltaY > 0 ? -0.08 : 0.08;
  state.zoomTarget = clampRange(state.zoomTarget + delta, 0.7, 1.5);
  state.zoomReturnAt = performance.now() + 5000;
}, { passive: false });
document.addEventListener("visibilitychange", () => {
  if (!document.hidden) {
    ensureSocketConnection();
  }
});
window.addEventListener("pagehide", () => {
  notifyLeave();
});
window.addEventListener("beforeunload", () => {
  notifyLeave();
});

renderCellOptions();

loginForm.addEventListener("submit", async (event) => {
  event.preventDefault();
  const nickname = nicknameInput.value.trim().slice(0, 16);
  if (!nickname) {
    showMessage("닉네임을 입력해 주세요.");
    return;
  }

  try {
    const response = await fetch("/api/join", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ nickname, cellType: state.selectedCellType }),
    });
    if (!response.ok) {
      throw new Error("join failed");
    }

    const data = await response.json();
    state.nickname = data.nickname;
    state.playerId = data.playerId;
    state.sessionId = data.sessionId;
    hudName.textContent = data.nickname;
    const selected = CELL_TYPES[data.cellType] || CELL_TYPES.classic;
    hudCellType.textContent = selected.name;
    hudAbilityName.textContent = selected.abilityName;
    connectSocket();
  } catch {
    showMessage("서버 연결에 실패했습니다.");
  }
});

function resizeCanvas() {
  canvas.width = window.innerWidth;
  canvas.height = window.innerHeight;
}

function connectSocket() {
  if (!state.playerId || !state.sessionId) {
    return;
  }
  if (state.socket && (state.socket.readyState === WebSocket.OPEN || state.socket.readyState === WebSocket.CONNECTING)) {
    return;
  }

  const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
  const socket = new WebSocket(`${protocol}//${window.location.host}/ws?playerId=${encodeURIComponent(state.playerId)}&sessionId=${encodeURIComponent(state.sessionId)}`);
  state.socket = socket;

  socket.addEventListener("open", () => {
    state.connected = true;
    state.reconnectAttempts = 0;
    clearReconnectTimer();
    loginScreen.classList.add("hidden");
    hud.classList.remove("hidden");
    leaderboard.classList.remove("hidden");
    minimap.classList.remove("hidden");
    if (!state.inputTimer) {
      state.inputTimer = setInterval(sendInput, 33);
    }
    if (!state.lastFrame) {
      requestAnimationFrame(loop);
    }
  });

  socket.addEventListener("message", (event) => {
    const data = JSON.parse(event.data);
    if (data.type === "snapshot") {
      state.lastSnapshotAt = performance.now();
      state.players = data.players;
      state.foods = data.foods;
      state.cacti = data.cacti || [];
      syncRenderPlayers(data.players);
      const me = state.renderPlayers.get(state.playerId);
      if (me) {
        hudMass.textContent = Math.round(me.mass);
        updateAbilityHud(me);
      }
      renderLeaderboard();
    }
  });

  socket.addEventListener("close", () => {
    state.connected = false;
    if (state.socket === socket) {
      state.socket = null;
    }
    scheduleReconnect();
  });

  socket.addEventListener("error", () => {
    state.connected = false;
    if (state.socket === socket) {
      state.socket = null;
    }
    scheduleReconnect();
  });
}

function sendInput() {
  if (!state.connected || !state.socket || state.socket.readyState !== WebSocket.OPEN) {
    return;
  }

  const centerX = canvas.width / 2;
  const centerY = canvas.height / 2;
  const dx = state.mouse.x - centerX;
  const dy = state.mouse.y - centerY;
  const length = Math.hypot(dx, dy) || 1;
  state.pendingDirection.x = dx / length;
  state.pendingDirection.y = dy / length;

  state.socket.send(JSON.stringify({
    type: "input",
    direction: state.pendingDirection,
    useAbility: state.abilityPressed,
    useSplit: state.splitPressed,
  }));
  state.abilityPressed = false;
  state.splitPressed = false;
}

function loop(timestamp) {
  const dt = Math.min(0.033, (timestamp - state.lastFrame) / 1000 || 0.016);
  state.lastFrame = timestamp;
  state.time = timestamp * 0.001;

  if (state.connected && state.lastSnapshotAt > 0 && timestamp - state.lastSnapshotAt > 7000) {
    forceReconnect();
  }

  updateZoom(timestamp);
  stepRenderPlayers();
  updateCamera();
  render();

  if (state.messageTimer > 0) {
    state.messageTimer -= dt;
    if (state.messageTimer <= 0) {
      messageBox.classList.add("hidden");
    }
  }

  requestAnimationFrame(loop);
}

function syncRenderPlayers(nextPlayers) {
  const nextIds = new Set();
  for (const player of nextPlayers) {
    nextIds.add(player.id);
    const existing = state.renderPlayers.get(player.id);
    if (existing) {
      existing.x = player.x;
      existing.y = player.y;
      existing.mass = player.mass;
      existing.radius = player.radius;
      existing.nickname = player.nickname;
      existing.color = player.color;
      existing.cellType = player.cellType;
      existing.abilityName = player.abilityName;
      existing.cooldownRemaining = player.cooldownRemaining;
      existing.effectRemaining = player.effectRemaining;
      existing.scale = player.scale;
      existing.isBot = player.isBot;
    } else {
      state.renderPlayers.set(player.id, {
        ...player,
        drawX: player.x,
        drawY: player.y,
        drawRadius: player.radius,
      });
    }
  }

  for (const id of [...state.renderPlayers.keys()]) {
    if (!nextIds.has(id)) {
      state.renderPlayers.delete(id);
    }
  }
}

function stepRenderPlayers() {
  for (const player of state.renderPlayers.values()) {
    const follow = player.id === state.playerId ? 0.42 : 0.2;
    player.drawX = lerp(player.drawX, player.x, follow);
    player.drawY = lerp(player.drawY, player.y, follow);
    player.drawRadius = lerp(player.drawRadius, player.radius * (player.scale || 1), 0.22);
  }
}

function updateCamera() {
  const me = state.renderPlayers.get(state.playerId);
  if (!me) {
    return;
  }
  state.camera.x = lerp(state.camera.x, me.drawX, 0.16);
  state.camera.y = lerp(state.camera.y, me.drawY, 0.16);
}

function render() {
  ctx.clearRect(0, 0, canvas.width, canvas.height);
  ctx.save();
  ctx.translate(canvas.width / 2, canvas.height / 2);
  ctx.scale(state.zoom, state.zoom);
  ctx.translate(-canvas.width / 2, -canvas.height / 2);
  drawBackground();
  drawCacti();
  drawFoods();
  drawPlayers();
  ctx.restore();
  drawMinimap();
}

function drawBackground() {
  ctx.fillStyle = "#08101d";
  ctx.fillRect(0, 0, canvas.width, canvas.height);

  ctx.save();
  ctx.translate(canvas.width / 2 - state.camera.x, canvas.height / 2 - state.camera.y);
  ctx.strokeStyle = "rgba(255,255,255,0.05)";
  for (let x = 0; x <= world.width; x += 120) {
    ctx.beginPath();
    ctx.moveTo(x, 0);
    ctx.lineTo(x, world.height);
    ctx.stroke();
  }
  for (let y = 0; y <= world.height; y += 120) {
    ctx.beginPath();
    ctx.moveTo(0, y);
    ctx.lineTo(world.width, y);
    ctx.stroke();
  }
  ctx.restore();
}

function drawFoods() {
  ctx.save();
  ctx.translate(canvas.width / 2 - state.camera.x, canvas.height / 2 - state.camera.y);
  for (const food of state.foods) {
    ctx.fillStyle = "#8affcf";
    ctx.beginPath();
    ctx.arc(food.x, food.y, food.radius, 0, Math.PI * 2);
    ctx.fill();
  }
  ctx.restore();
}

function drawCacti() {
  ctx.save();
  ctx.translate(canvas.width / 2 - state.camera.x, canvas.height / 2 - state.camera.y);

  for (const cactus of state.cacti) {
    const phase = state.time * 2.2 + cactus.x * 0.01 + cactus.y * 0.008;
    const pulse = 1 + Math.sin(phase) * 0.05;
    const radius = cactus.size * pulse;
    const spikes = 16;

    ctx.beginPath();
    for (let i = 0; i < spikes * 2; i += 1) {
      const angle = (Math.PI * i) / spikes;
      const wobble = 1 + Math.sin(phase * 1.6 + i * 0.9) * 0.08;
      const spikeRadius = (i % 2 === 0 ? radius * 1.28 : radius * 0.92) * wobble;
      const x = cactus.x + Math.cos(angle) * spikeRadius;
      const y = cactus.y + Math.sin(angle) * spikeRadius;
      if (i === 0) {
        ctx.moveTo(x, y);
      } else {
        ctx.lineTo(x, y);
      }
    }
    ctx.closePath();

    const gradient = ctx.createRadialGradient(
      cactus.x - radius * 0.25,
      cactus.y - radius * 0.35,
      radius * 0.2,
      cactus.x,
      cactus.y,
      radius * 1.3,
    );
    gradient.addColorStop(0, "#8dff5c");
    gradient.addColorStop(0.55, "#41cf37");
    gradient.addColorStop(1, "#209327");
    ctx.fillStyle = gradient;
    ctx.fill();

    ctx.strokeStyle = "#b9ff84";
    ctx.lineWidth = 2;
    ctx.stroke();

    ctx.fillStyle = "rgba(255,255,255,0.18)";
    ctx.beginPath();
    ctx.arc(
      cactus.x - radius * 0.2 + Math.cos(phase * 1.4) * radius * 0.08,
      cactus.y - radius * 0.24 + Math.sin(phase * 1.1) * radius * 0.08,
      radius * 0.24,
      0,
      Math.PI * 2,
    );
    ctx.fill();

    ctx.fillStyle = "rgba(0, 0, 0, 0.16)";
    ctx.beginPath();
    ctx.arc(cactus.x, cactus.y, radius * 0.52, 0, Math.PI * 2);
    ctx.fill();

    ctx.strokeStyle = "rgba(185,255,132,0.35)";
    ctx.lineWidth = 1.4;
    ctx.beginPath();
    ctx.arc(cactus.x, cactus.y, radius * (1.04 + Math.sin(phase * 1.9) * 0.03), 0, Math.PI * 2);
    ctx.stroke();
  }

  ctx.restore();
}

function drawPlayers() {
  const me = state.renderPlayers.get(state.playerId);
  ctx.save();
  ctx.translate(canvas.width / 2 - state.camera.x, canvas.height / 2 - state.camera.y);

  for (const player of state.renderPlayers.values()) {
    const isMe = player.id === state.playerId;
    const canEat = me && me.id !== player.id && me.mass > player.mass * 1.1;
    const canBeEaten = me && me.id !== player.id && player.mass > me.mass * 1.1;

    ctx.fillStyle = isMe ? "#60b9ff" : canEat ? "#8affcf" : canBeEaten ? "#ff8b9d" : player.color;
    ctx.beginPath();
    ctx.arc(player.drawX, player.drawY, player.drawRadius, 0, Math.PI * 2);
    ctx.fill();

    ctx.strokeStyle = isMe ? "#ffffff" : "rgba(255,255,255,0.35)";
    ctx.lineWidth = isMe ? 3 : 1.5;
    ctx.stroke();

    if (player.effectRemaining > 0) {
      ctx.strokeStyle = "rgba(255, 205, 112, 0.9)";
      ctx.lineWidth = 3;
      ctx.beginPath();
      ctx.arc(player.drawX, player.drawY, player.drawRadius + 8, 0, Math.PI * 2);
      ctx.stroke();
    }

    ctx.fillStyle = "#eef7ff";
    ctx.font = "14px Segoe UI";
    ctx.textAlign = "center";
    ctx.fillText(player.nickname, player.drawX, player.drawY - player.drawRadius - 10);
    ctx.fillText(String(Math.round(player.mass)), player.drawX, player.drawY + 5);
  }

  ctx.restore();
}

function renderLeaderboard() {
  const sorted = [...state.players].sort((a, b) => b.mass - a.mass);
  const myIndex = sorted.findIndex((player) => player.id === state.playerId);
  hudRank.textContent = myIndex >= 0 ? `${myIndex + 1} / ${sorted.length}` : "-";
  leaderboard.innerHTML = `
    <h2>리더보드</h2>
    ${sorted.slice(0, 6).map((player, index) => `
      <div class="leader-line">
        <span>${index + 1}. ${player.nickname}</span>
        <strong>${Math.round(player.mass)}</strong>
      </div>
    `).join("")}
  `;
}

function showMessage(text) {
  messageBox.textContent = text;
  messageBox.classList.remove("hidden");
  state.messageTimer = 2.2;
}

function ensureSocketConnection() {
  if (!state.playerId || !state.sessionId) {
    return;
  }
  if (!state.socket || state.socket.readyState === WebSocket.CLOSED || state.socket.readyState === WebSocket.CLOSING) {
    connectSocket();
  }
}

function scheduleReconnect() {
  if (!state.playerId || !state.sessionId || state.reconnectTimer) {
    return;
  }
  showMessage("연결이 끊겨 재연결 중입니다.");
  state.reconnectTimer = window.setTimeout(() => {
    state.reconnectTimer = null;
    state.reconnectAttempts += 1;
    if (state.reconnectAttempts >= 2) {
      rejoinSession();
      return;
    }
    connectSocket();
    scheduleReconnect();
  }, 1200);
}

function clearReconnectTimer() {
  if (state.reconnectTimer) {
    clearTimeout(state.reconnectTimer);
    state.reconnectTimer = null;
  }
}

function forceReconnect() {
  if (state.socket && (state.socket.readyState === WebSocket.OPEN || state.socket.readyState === WebSocket.CONNECTING)) {
    try {
      state.socket.close();
    } catch {
      // noop
    }
    state.socket = null;
  } else {
    state.connected = false;
    scheduleReconnect();
  }
}

async function rejoinSession() {
  clearReconnectTimer();
  if (!state.nickname) {
    connectSocket();
    return;
  }

  showMessage("세션이 만료되어 자동 재입장 중입니다.");

  try {
    const response = await fetch("/api/join", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ nickname: state.nickname, cellType: state.selectedCellType }),
    });
    if (!response.ok) {
      throw new Error("rejoin failed");
    }

    const data = await response.json();
    state.nickname = data.nickname;
    state.socket = null;
    state.playerId = data.playerId;
    state.sessionId = data.sessionId;
    state.reconnectAttempts = 0;
    connectSocket();
  } catch {
    state.reconnectTimer = window.setTimeout(() => {
      state.reconnectTimer = null;
      rejoinSession();
    }, 2000);
  }
}

function renderCellOptions() {
  cellOptions.innerHTML = Object.entries(CELL_TYPES).map(([key, cell]) => `
    <article class="cell-option ${key === state.selectedCellType ? "selected" : ""}" data-cell-type="${key}">
      <h3>${cell.name}</h3>
      <p>${cell.description}</p>
      <div class="cell-meta">${cell.abilityName} · ${cell.detail}</div>
    </article>
  `).join("");

  cellOptions.querySelectorAll(".cell-option").forEach((element) => {
    element.addEventListener("click", () => {
      state.selectedCellType = element.dataset.cellType;
      renderCellOptions();
    });
  });
}

function notifyLeave() {
  if (!state.playerId || !state.sessionId) {
    return;
  }

  try {
    const payload = JSON.stringify({
      playerId: state.playerId,
      sessionId: state.sessionId,
    });
    navigator.sendBeacon("/api/leave", new Blob([payload], { type: "application/json" }));
  } catch {
    // noop
  }
}

function updateAbilityHud(player) {
  const cell = CELL_TYPES[player.cellType] || CELL_TYPES.classic;
  hudCellType.textContent = cell.name || player.cellType;
  hudAbilityName.textContent = player.abilityName || "없음";

  const cooldownRatio = cell.cooldownMs > 0 ? clamp01(1 - (player.cooldownRemaining / cell.cooldownMs)) : 1;
  hudCooldownFill.style.width = `${cooldownRatio * 100}%`;
  hudCooldownLabel.textContent = player.cooldownRemaining > 0 ? "재충전" : "준비";

  const effectRatio = cell.effectMs > 0 ? clamp01(player.effectRemaining / cell.effectMs) : 0;
  hudEffectFill.style.width = `${effectRatio * 100}%`;
  hudEffectLabel.textContent = player.effectRemaining > 0 ? "활성" : "없음";
}

function drawMinimap() {
  const width = minimapCanvas.width;
  const height = minimapCanvas.height;
  const me = state.players.find((player) => player.id === state.playerId);
  if (!me) {
    return;
  }

  minimapCtx.clearRect(0, 0, width, height);
  minimapCtx.fillStyle = "#091120";
  minimapCtx.fillRect(0, 0, width, height);

  minimapCtx.strokeStyle = "rgba(255,255,255,0.12)";
  minimapCtx.strokeRect(0.5, 0.5, width - 1, height - 1);

  const range = MINIMAP_RANGE;
  const scaleX = width / world.width;
  const scaleY = height / world.height;

  minimapCtx.strokeStyle = "rgba(138,255,207,0.18)";
  const visionWidth = range * 2 * scaleX;
  const visionHeight = range * 2 * scaleY;
  minimapCtx.strokeRect(
    clampRange(me.x * scaleX - visionWidth / 2, 0, width - visionWidth),
    clampRange(me.y * scaleY - visionHeight / 2, 0, height - visionHeight),
    Math.min(visionWidth, width),
    Math.min(visionHeight, height),
  );

  minimapCtx.fillStyle = "rgba(138,255,207,0.35)";
  for (const food of state.foods) {
    const dx = food.x - me.x;
    const dy = food.y - me.y;
    if (Math.abs(dx) > range || Math.abs(dy) > range) {
      continue;
    }
    const x = food.x * scaleX;
    const y = food.y * scaleY;
    minimapCtx.fillRect(x, y, 2, 2);
  }

  for (const player of state.players) {
    const isMe = player.id === state.playerId;
    const dx = player.x - me.x;
    const dy = player.y - me.y;
    if (!isMe && (Math.abs(dx) > range || Math.abs(dy) > range)) {
      continue;
    }
    const x = player.x * scaleX;
    const y = player.y * scaleY;
    minimapCtx.fillStyle = isMe ? "#ffffff" : player.isBot ? "rgba(186,205,255,0.85)" : "rgba(96,185,255,0.85)";
    minimapCtx.beginPath();
    minimapCtx.arc(x, y, isMe ? 4 : 2.5, 0, Math.PI * 2);
    minimapCtx.fill();
  }
}

function lerp(start, end, alpha) {
  return start + (end - start) * alpha;
}

function clamp01(value) {
  return Math.max(0, Math.min(1, value));
}

function clampRange(value, min, max) {
  return Math.max(min, Math.min(max, value));
}

function updateZoom(timestamp) {
  if (state.zoomReturnAt > 0 && timestamp >= state.zoomReturnAt) {
    state.zoomTarget = 1;
    state.zoomReturnAt = 0;
  }
  state.zoom = lerp(state.zoom, state.zoomTarget, 0.12);
}

resizeCanvas();
