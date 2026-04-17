const canvas = document.getElementById("gameCanvas");
const ctx = canvas.getContext("2d");

const loginScreen = document.getElementById("loginScreen");
const loginForm = document.getElementById("loginForm");
const nicknameInput = document.getElementById("nicknameInput");
const cellOptions = document.getElementById("cellOptions");
const hud = document.getElementById("hud");
const leaderboard = document.getElementById("leaderboard");
const leaderboardToggle = document.getElementById("leaderboardToggle");
const leaderboardContent = document.getElementById("leaderboardContent");
const minimap = document.getElementById("minimap");
const minimapToggle = document.getElementById("minimapToggle");
const minimapCanvas = document.getElementById("minimapCanvas");
const minimapCtx = minimapCanvas.getContext("2d");
const chatPanel = document.getElementById("chatPanel");
const chatToggle = document.getElementById("chatToggle");
const chatBody = document.getElementById("chatBody");
const chatMessages = document.getElementById("chatMessages");
const chatForm = document.getElementById("chatForm");
const chatInput = document.getElementById("chatInput");
const touchControls = document.getElementById("touchControls");
const touchPad = document.getElementById("touchPad");
const touchStick = document.getElementById("touchStick");
const touchAbility = document.getElementById("touchAbility");
const touchSplit = document.getElementById("touchSplit");
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
    name: "오버클럭",
    abilityName: "코어 가속",
    description: "스페이스바를 누르는 동안 에너지를 소모해 지속적으로 가속합니다. 사용을 멈추면 에너지가 서서히 자동 충전됩니다.",
    detail: "최대 1.5초 가속 / 4초 완충",
    cooldownMs: 4000,
    effectMs: 1500,
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
  divider: {
    name: "분열 세포",
    abilityName: "세포 분열",
    description: "전용기를 쓰면 현재 조각들이 반으로 갈라지고, 시간이 지나 가까운 조각끼리는 다시 합쳐집니다.",
    detail: "약 1.4초 쿨타임 / 반분열 / 자동 재결합",
    cooldownMs: 1400,
    effectMs: 0,
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
  leaderboard: [],
  chats: [],
  renderPlayers: new Map(),
  foods: [],
  cacti: [],
  wormholes: [],
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
  snapshotGap: 33,
  time: 0,
  leaderboardCollapsed: false,
  minimapCollapsed: false,
  chatCollapsed: false,
  chatLastActivityAt: 0,
  isTouchDevice: matchMedia("(pointer: coarse)").matches || "ontouchstart" in window,
  touch: {
    active: false,
    pointerId: null,
    originX: 0,
    originY: 0,
    dx: 0,
    dy: 0,
    radius: 44,
  },
};

function isTypingInField() {
  const active = document.activeElement;
  return active === nicknameInput || active === chatInput;
}

if (state.isTouchDevice) {
  document.body.classList.add("touch-device");
}

window.addEventListener("resize", resizeCanvas);
window.addEventListener("keydown", (event) => {
  if (isTypingInField()) {
    return;
  }
  if (event.code === "Space") { // 꾹 누를 때 연속 입력 허용
    state.abilityPressed = true;
  }
  if (event.code === "KeyW" && !event.repeat) {
    state.splitPressed = true;
  }
  if (event.code === "Enter" && state.connected) {
    event.preventDefault();
    chatInput.focus();
  }
});

// ✅ 새로 추가: 키를 뗄 때 가속 중지
window.addEventListener("keyup", (event) => {
  if (isTypingInField()) {
    return;
  }
  if (event.code === "Space") {
    state.abilityPressed = false;
  }
});
canvas.addEventListener("mousemove", (event) => {
  if (state.touch.active) {
    return;
  }
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
    state.lastSnapshotAt = performance.now(); // 탭에 복귀할 때 타이머를 초기화하여 억울한 튕김 방지
    ensureSocketConnection();
  }
});
window.addEventListener("pagehide", () => {
  notifyLeave();
});
window.addEventListener("beforeunload", () => {
  notifyLeave();
});
canvas.addEventListener("pointerdown", onTouchPadDown);
window.addEventListener("pointermove", onTouchPadMove);
window.addEventListener("pointerup", onTouchPadEnd);
window.addEventListener("pointercancel", onTouchPadEnd);
touchAbility.addEventListener("pointerdown", (event) => {
  event.preventDefault();
  state.abilityPressed = true;
});

// ✅ 새로 추가: 손가락을 떼거나 빗나갔을 때
touchAbility.addEventListener("pointerup", (event) => {
  event.preventDefault();
  state.abilityPressed = false;
});
touchAbility.addEventListener("pointerleave", (event) => {
  event.preventDefault();
  state.abilityPressed = false;
});
touchAbility.addEventListener("pointercancel", (event) => {
  event.preventDefault();
  state.abilityPressed = false;
});
touchSplit.addEventListener("pointerdown", (event) => {
  event.preventDefault();
  state.splitPressed = true;
});
touchSplit.textContent = "W";
minimapToggle.addEventListener("click", () => {
  state.minimapCollapsed = !state.minimapCollapsed;
  minimap.classList.toggle("collapsed", state.minimapCollapsed);
  minimapToggle.textContent = state.minimapCollapsed ? "지도 열기" : "지도 접기";
  minimapToggle.setAttribute("aria-expanded", String(!state.minimapCollapsed));
});
leaderboardToggle.addEventListener("click", () => {
  state.leaderboardCollapsed = !state.leaderboardCollapsed;
  leaderboard.classList.toggle("collapsed", state.leaderboardCollapsed);
  leaderboardToggle.textContent = state.leaderboardCollapsed ? "순위 열기" : "순위 접기";
  leaderboardToggle.setAttribute("aria-expanded", String(!state.leaderboardCollapsed));
});
chatToggle.addEventListener("click", () => {
  state.chatCollapsed = !state.chatCollapsed;
  chatPanel.classList.toggle("collapsed", state.chatCollapsed);
  chatToggle.textContent = state.chatCollapsed ? "채팅 열기" : "채팅 접기";
  chatToggle.setAttribute("aria-expanded", String(!state.chatCollapsed));
  if (!state.chatCollapsed) {
    markChatActivity();
  }
});

chatForm.addEventListener("submit", (event) => {
  event.preventDefault();
  sendChat();
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
    chatPanel.classList.remove("hidden");
    if (state.isTouchDevice) {
      touchControls.classList.remove("hidden");
    }
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
      const snapshotAt = performance.now();
      if (state.lastSnapshotAt > 0) {
        state.snapshotGap = Math.max(16, Math.min(140, snapshotAt - state.lastSnapshotAt));
      }
      state.lastSnapshotAt = snapshotAt;
      state.players = data.players;
      state.leaderboard = data.leaderboard || [];
      const nextChats = data.chats || [];
      if (nextChats.length !== state.chats.length) {
        markChatActivity();
      } else if (nextChats.length > 0) {
        const prevLast = state.chats[state.chats.length - 1];
        const nextLast = nextChats[nextChats.length - 1];
        if (!prevLast || !nextLast || prevLast.id !== nextLast.id) {
          markChatActivity();
        }
      }
      state.chats = nextChats;
      state.foods = data.foods;
      state.cacti = data.cacti || [];
      state.wormholes = data.wormholes || [];
      syncRenderPlayers(data.players, snapshotAt);
      const me = state.renderPlayers.get(state.playerId);
      const grouped = state.leaderboard.length > 0 ? state.leaderboard : aggregateOwners(data.players);
      const myOwnerId = me ? (me.ownerId || me.id) : state.playerId;
      const myGroup = grouped.find((entry) => entry.ownerId === myOwnerId);
      if (me) {
        hudMass.textContent = Math.round(myGroup ? myGroup.mass : me.mass);
        updateAbilityHud(me);
      }
      renderLeaderboard();
      renderChat();
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
  if (state.touch.active) {
    state.pendingDirection.x = state.touch.dx;
    state.pendingDirection.y = state.touch.dy;
  } else {
    const dx = state.mouse.x - centerX;
    const dy = state.mouse.y - centerY;
    const length = Math.hypot(dx, dy) || 1;
    state.pendingDirection.x = dx / length;
    state.pendingDirection.y = dy / length;
  }

  state.socket.send(JSON.stringify({
    type: "input",
    direction: state.pendingDirection,
    useAbility: state.abilityPressed,
    useSplit: state.splitPressed,
  }));
  state.splitPressed = false;
}

function sendChat() {
  const text = chatInput.value.trim().slice(0, 96);
  if (!text || !state.connected || !state.socket || state.socket.readyState !== WebSocket.OPEN) {
    return;
  }

  state.socket.send(JSON.stringify({
    type: "chat",
    message: text,
  }));
  chatInput.value = "";
  markChatActivity();
  if (state.isTouchDevice) {
    chatInput.blur();
  }
}

function loop(timestamp) {
  const dt = Math.min(0.033, (timestamp - state.lastFrame) / 1000 || 0.016);
  state.lastFrame = timestamp;
  state.time = timestamp * 0.001;

  if (!document.hidden && state.connected && state.lastSnapshotAt > 0 && timestamp - state.lastSnapshotAt > 7000) {
    forceReconnect();
  }

  updateZoom(timestamp);
  stepRenderPlayers(dt, timestamp);
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

function syncRenderPlayers(nextPlayers, snapshotAt) {
  const nextIds = new Set();
  for (const player of nextPlayers) {
    nextIds.add(player.id);
    const scaledRadius = player.radius * (player.scale || 1);
    const existing = state.renderPlayers.get(player.id);
    if (existing) {
      const previousServerX = existing.serverX ?? existing.x;
      const previousServerY = existing.serverY ?? existing.y;
      const previousServerRadius = existing.serverRadius ?? existing.radius;
      const gap = Math.max(16, snapshotAt - (existing.snapshotAt || snapshotAt - state.snapshotGap));

      existing.prevServerX = previousServerX;
      existing.prevServerY = previousServerY;
      existing.prevServerRadius = previousServerRadius;
      existing.serverX = player.x;
      existing.serverY = player.y;
      existing.serverRadius = scaledRadius;
      existing.snapshotAt = snapshotAt;
      existing.snapshotGap = gap;
      existing.velocityX = (existing.serverX - previousServerX) / (gap / 1000);
      existing.velocityY = (existing.serverY - previousServerY) / (gap / 1000);
      existing.x = player.x;
      existing.y = player.y;
      existing.mass = player.mass;
      existing.radius = player.radius;
      existing.ownerId = player.ownerId;
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
        drawRadius: scaledRadius,
        prevServerX: player.x,
        prevServerY: player.y,
        prevServerRadius: scaledRadius,
        serverX: player.x,
        serverY: player.y,
        serverRadius: scaledRadius,
        snapshotAt,
        snapshotGap: state.snapshotGap,
        velocityX: 0,
        velocityY: 0,
      });
    }
  }

  for (const id of [...state.renderPlayers.keys()]) {
    if (!nextIds.has(id)) {
      state.renderPlayers.delete(id);
    }
  }
}

function stepRenderPlayers(dt, timestamp) {
  for (const player of state.renderPlayers.values()) {
    const gap = Math.max(16, player.snapshotGap || state.snapshotGap || 33);
    const snapshotAge = Math.max(0, timestamp - (player.snapshotAt || timestamp));
    const blend = clampRange(snapshotAge / gap, 0, 1.18);
    const interpolatedX = lerp(player.prevServerX ?? player.serverX, player.serverX ?? player.x, blend);
    const interpolatedY = lerp(player.prevServerY ?? player.serverY, player.serverY ?? player.y, blend);
    const interpolatedRadius = lerp(player.prevServerRadius ?? player.serverRadius, player.serverRadius ?? player.radius, clampRange(blend, 0, 1));
    const extrapolation = Math.min(90, Math.max(0, snapshotAge - gap * 0.45)) / 1000;
    const targetX = interpolatedX + (player.velocityX || 0) * extrapolation * 0.35;
    const targetY = interpolatedY + (player.velocityY || 0) * extrapolation * 0.35;
    const positionSharpness = player.id === state.playerId ? 16 : 11;
    const radiusSharpness = player.id === state.playerId ? 14 : 10;

    player.drawX = smoothTowards(player.drawX, targetX, positionSharpness, dt);
    player.drawY = smoothTowards(player.drawY, targetY, positionSharpness, dt);
    player.drawRadius = smoothTowards(player.drawRadius, interpolatedRadius, radiusSharpness, dt);
  }
}

function updateCamera() {
  const center = getOwnedCenterFromRenderPlayers();
  if (!center) {
    return;
  }
  state.camera.x = lerp(state.camera.x, center.x, 0.16);
  state.camera.y = lerp(state.camera.y, center.y, 0.16);
}

function render() {
  ctx.clearRect(0, 0, canvas.width, canvas.height);
  ctx.save();
  ctx.translate(canvas.width / 2, canvas.height / 2);
  ctx.scale(state.zoom, state.zoom);
  ctx.translate(-canvas.width / 2, -canvas.height / 2);
  drawBackground();
  drawWormholes();
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

function drawWormholes() {
  ctx.save();
  ctx.translate(canvas.width / 2 - state.camera.x, canvas.height / 2 - state.camera.y);

  for (const hole of state.wormholes) {
    const phase = state.time * (hole.kind === "blackhole" ? 2.8 : 2.2) + hole.x * 0.006;
    const radius = hole.radius * (1 + Math.sin(phase) * 0.035);

    ctx.save();
    ctx.translate(hole.x, hole.y);

    if (hole.kind === "blackhole") {
      const outer = ctx.createRadialGradient(0, 0, radius * 0.12, 0, 0, hole.pullRange);
      outer.addColorStop(0, "rgba(6, 12, 22, 0.96)");
      outer.addColorStop(0.5, "rgba(71, 92, 170, 0.24)");
      outer.addColorStop(1, "rgba(71, 92, 170, 0)");
      ctx.fillStyle = outer;
      ctx.beginPath();
      ctx.arc(0, 0, hole.pullRange, 0, Math.PI * 2);
      ctx.fill();

      ctx.strokeStyle = "rgba(112, 146, 255, 0.45)";
      ctx.lineWidth = 2;
      ctx.beginPath();
      ctx.arc(0, 0, hole.pullRange * 0.72, 0, Math.PI * 2);
      ctx.stroke();

      for (let ring = 0; ring < 3; ring += 1) {
        ctx.strokeStyle = `rgba(134, 164, 255, ${0.22 - ring * 0.05})`;
        ctx.lineWidth = 3 - ring * 0.6;
        ctx.beginPath();
        ctx.ellipse(0, 0, radius * (1.1 + ring * 0.3), radius * (0.72 + ring * 0.18), phase + ring * 0.8, 0, Math.PI * 2);
        ctx.stroke();
      }

      const core = ctx.createRadialGradient(-radius * 0.15, -radius * 0.12, radius * 0.1, 0, 0, radius);
      core.addColorStop(0, "#87a2ff");
      core.addColorStop(0.25, "#2e3978");
      core.addColorStop(1, "#04070c");
      ctx.fillStyle = core;
    } else {
      const outer = ctx.createRadialGradient(0, 0, radius * 0.1, 0, 0, hole.pullRange * 0.75);
      outer.addColorStop(0, "rgba(255, 255, 255, 0.95)");
      outer.addColorStop(0.35, "rgba(170, 255, 242, 0.52)");
      outer.addColorStop(1, "rgba(170, 255, 242, 0)");
      ctx.fillStyle = outer;
      ctx.beginPath();
      ctx.arc(0, 0, hole.pullRange * 0.75, 0, Math.PI * 2);
      ctx.fill();

      for (let ring = 0; ring < 3; ring += 1) {
        ctx.strokeStyle = `rgba(208, 255, 247, ${0.42 - ring * 0.09})`;
        ctx.lineWidth = 2.8 - ring * 0.5;
        ctx.beginPath();
        ctx.arc(0, 0, radius * (1 + ring * 0.36 + Math.sin(phase + ring) * 0.05), 0, Math.PI * 2);
        ctx.stroke();
      }

      const core = ctx.createRadialGradient(-radius * 0.22, -radius * 0.18, radius * 0.14, 0, 0, radius * 1.08);
      core.addColorStop(0, "#ffffff");
      core.addColorStop(0.35, "#9dfff0");
      core.addColorStop(1, "#2dd0ff");
      ctx.fillStyle = core;
    }

    ctx.beginPath();
    ctx.arc(0, 0, radius, 0, Math.PI * 2);
    ctx.fill();

    ctx.restore();
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
    const isSameOwner = me && (me.ownerId || me.id) === (player.ownerId || player.id);
    const canEat = me && !isSameOwner && me.id !== player.id && me.mass > player.mass * 1.1;
    const canBeEaten = me && !isSameOwner && me.id !== player.id && player.mass > me.mass * 1.1;

    ctx.fillStyle = player.color;
    ctx.beginPath();
    ctx.arc(player.drawX, player.drawY, player.drawRadius, 0, Math.PI * 2);
    ctx.fill();

    ctx.strokeStyle = isMe
      ? "#ffffff"
      : isSameOwner
        ? "rgba(255,255,255,0.72)"
        : canEat
          ? "rgba(138,255,207,0.85)"
          : canBeEaten
            ? "rgba(255,139,157,0.85)"
            : "rgba(255,255,255,0.35)";
    ctx.lineWidth = isMe ? 3 : isSameOwner ? 2.4 : 1.5;
    ctx.stroke();

    if (player.effectRemaining > 0) {
      ctx.strokeStyle = "rgba(255, 205, 112, 0.9)";
      ctx.lineWidth = 3;
      ctx.beginPath();
      ctx.arc(player.drawX, player.drawY, player.drawRadius + 8, 0, Math.PI * 2);
      ctx.stroke();
    }

    ctx.fillStyle = "#eef7ff";
    const nameFontSize = Math.max(11, Math.min(18, player.drawRadius * 0.38));
    const massFontSize = Math.max(10, Math.min(15, player.drawRadius * 0.28));
    ctx.textAlign = "center";
    ctx.textBaseline = "middle";
    ctx.font = `700 ${nameFontSize}px Segoe UI`;
    ctx.fillText(player.nickname, player.drawX, player.drawY - Math.min(8, player.drawRadius * 0.16));
    ctx.font = `${massFontSize}px Segoe UI`;
    ctx.fillStyle = "rgba(238,247,255,0.82)";
    ctx.fillText(String(Math.round(player.mass)), player.drawX, player.drawY + Math.min(12, player.drawRadius * 0.2));
  }

  ctx.restore();
}

function renderLeaderboard() {
  const grouped = (state.leaderboard.length > 0 ? [...state.leaderboard] : aggregateOwners(state.players).sort((a, b) => b.mass - a.mass));
  const myOwnerId = getMyOwnerId();
  const myIndex = grouped.findIndex((player) => player.ownerId === myOwnerId);
  hudRank.textContent = myIndex >= 0 ? `${myIndex + 1} / ${grouped.length}` : "-";
  leaderboardContent.innerHTML = `
    <h2>리더보드</h2>
    ${grouped.slice(0, 6).map((player, index) => `
      <div class="leader-line">
        <span>${index + 1}. ${player.nickname}</span>
        <strong>${Math.round(player.mass)}</strong>
      </div>
    `).join("")}
  `;
}

function renderChat() {
  const idleSeconds = (performance.now() - state.chatLastActivityAt) / 1000;
  chatPanel.classList.toggle("faded", !state.chatCollapsed && state.chats.length > 0 && idleSeconds > 4);
  const items = state.chats.slice(-12);
  if (items.length === 0) {
    chatMessages.innerHTML = `<div class="chat-entry"><div class="chat-text">아직 채팅이 없습니다.</div></div>`;
    return;
  }

  chatMessages.innerHTML = items.map((entry) => `
    <div class="chat-entry ${entry.isBot ? "bot" : ""}">
      <div class="chat-name">${escapeHtml(entry.nickname)}</div>
      <div class="chat-text">${escapeHtml(entry.message)}</div>
    </div>
  `).join("");
  chatMessages.scrollTop = chatMessages.scrollHeight;
}

function markChatActivity() {
  state.chatLastActivityAt = performance.now();
  chatPanel.classList.remove("faded");
}

function showMessage(text) {
  messageBox.textContent = text;
  messageBox.classList.remove("hidden");
  state.messageTimer = 2.2;
}

function escapeHtml(value) {
  return String(value)
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;");
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

function onTouchPadDown(event) {
  if (!state.isTouchDevice || event.pointerType === "mouse") {
    return;
  }
  if (event.clientX > window.innerWidth * 0.58) {
    return;
  }
  event.preventDefault();
  state.touch.active = true;
  state.touch.pointerId = event.pointerId;
  state.touch.originX = clampRange(event.clientX, 74, window.innerWidth - 74);
  state.touch.originY = clampRange(event.clientY, 74, window.innerHeight - 74);
  touchPad.style.left = `${state.touch.originX}px`;
  touchPad.style.top = `${state.touch.originY}px`;
  touchPad.classList.add("active");
  updateTouchVector(event.clientX, event.clientY);
}

function onTouchPadMove(event) {
  if (!state.touch.active || event.pointerId !== state.touch.pointerId) {
    return;
  }
  event.preventDefault();
  updateTouchVector(event.clientX, event.clientY);
}

function onTouchPadEnd(event) {
  if (!state.touch.active || event.pointerId !== state.touch.pointerId) {
    return;
  }
  event.preventDefault();
  state.touch.active = false;
  state.touch.pointerId = null;
  state.touch.dx = 0;
  state.touch.dy = 0;
  touchPad.classList.remove("active");
  touchStick.style.transform = "translate(-50%, -50%)";
}

function updateTouchVector(clientX, clientY) {
  const dx = clientX - state.touch.originX;
  const dy = clientY - state.touch.originY;
  const distance = Math.hypot(dx, dy);
  const maxDistance = state.touch.radius;
  const clampedDistance = Math.min(distance, maxDistance);
  const nx = distance > 0 ? dx / distance : 0;
  const ny = distance > 0 ? dy / distance : 0;
  const offsetX = nx * clampedDistance;
  const offsetY = ny * clampedDistance;

  state.touch.dx = distance > 0 ? offsetX / maxDistance : 0;
  state.touch.dy = distance > 0 ? offsetY / maxDistance : 0;
  touchStick.style.transform = `translate(calc(-50% + ${offsetX}px), calc(-50% + ${offsetY}px))`;
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

function aggregateOwners(players) {
  const totals = new Map();
  for (const player of players) {
    const ownerId = player.ownerId || player.id;
    const existing = totals.get(ownerId);
    if (existing) {
      existing.mass += player.mass;
      existing.fragments += 1;
      if (player.mass > existing.maxMass) {
        existing.maxMass = player.mass;
        existing.nickname = player.nickname;
      }
    } else {
      totals.set(ownerId, {
        ownerId,
        nickname: player.nickname,
        mass: player.mass,
        maxMass: player.mass,
        fragments: 1,
      });
    }
  }
  return [...totals.values()];
}

function getMyOwnerId() {
  const me = state.renderPlayers.get(state.playerId) || state.players.find((player) => player.id === state.playerId);
  return me ? (me.ownerId || me.id) : state.playerId;
}

function getOwnedCenterFromRenderPlayers() {
  const ownerId = getMyOwnerId();
  let totalMass = 0;
  let x = 0;
  let y = 0;

  for (const player of state.renderPlayers.values()) {
    if ((player.ownerId || player.id) !== ownerId) {
      continue;
    }
    const mass = Math.max(1, player.mass);
    x += player.drawX * mass;
    y += player.drawY * mass;
    totalMass += mass;
  }

  if (totalMass <= 0) {
    return null;
  }

  return { x: x / totalMass, y: y / totalMass };
}

function getOwnedCenterFromPlayers() {
  const ownerId = getMyOwnerId();
  let totalMass = 0;
  let x = 0;
  let y = 0;

  for (const player of state.players) {
    if ((player.ownerId || player.id) !== ownerId) {
      continue;
    }
    const mass = Math.max(1, player.mass);
    x += player.x * mass;
    y += player.y * mass;
    totalMass += mass;
  }

  if (totalMass <= 0) {
    return null;
  }

  return { x: x / totalMass, y: y / totalMass };
}

function drawMinimap() {
  if (state.minimapCollapsed) {
    return;
  }

  const width = minimapCanvas.width;
  const height = minimapCanvas.height;
  const center = getOwnedCenterFromPlayers();
  const myOwnerId = getMyOwnerId();
  if (!center) {
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
    clampRange(center.x * scaleX - visionWidth / 2, 0, width - visionWidth),
    clampRange(center.y * scaleY - visionHeight / 2, 0, height - visionHeight),
    Math.min(visionWidth, width),
    Math.min(visionHeight, height),
  );

  minimapCtx.fillStyle = "rgba(138,255,207,0.35)";
  for (const food of state.foods) {
    const dx = food.x - center.x;
    const dy = food.y - center.y;
    if (Math.abs(dx) > range || Math.abs(dy) > range) {
      continue;
    }
    const x = food.x * scaleX;
    const y = food.y * scaleY;
    minimapCtx.fillRect(x, y, 2, 2);
  }

  for (const player of state.players) {
    const isMine = (player.ownerId || player.id) === myOwnerId;
    const dx = player.x - center.x;
    const dy = player.y - center.y;
    if (!isMine && (Math.abs(dx) > range || Math.abs(dy) > range)) {
      continue;
    }
    const x = player.x * scaleX;
    const y = player.y * scaleY;
    minimapCtx.fillStyle = isMine ? "#ffffff" : player.isBot ? "rgba(186,205,255,0.85)" : "rgba(96,185,255,0.85)";
    minimapCtx.beginPath();
    minimapCtx.arc(x, y, isMine ? 4 : 2.5, 0, Math.PI * 2);
    minimapCtx.fill();
  }
}

function lerp(start, end, alpha) {
  return start + (end - start) * alpha;
}

function smoothTowards(current, target, sharpness, dt) {
  return lerp(current, target, 1 - Math.exp(-sharpness * dt));
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
