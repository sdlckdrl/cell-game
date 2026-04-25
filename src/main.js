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
const touchMerge = document.getElementById("touchMerge");
const touchSplit = document.getElementById("touchSplit");
const fullscreenPrompt = document.getElementById("fullscreenPrompt");
const fullscreenAccept = document.getElementById("fullscreenAccept");
const fullscreenDismiss = document.getElementById("fullscreenDismiss");
const rotatePrompt = document.getElementById("rotatePrompt");
const resetNotice = document.getElementById("resetNotice");
const messageBox = document.getElementById("message");
const chatToast = document.getElementById("chatToast");
const hudName = document.getElementById("hudName");
const hudCellType = document.getElementById("hudCellType");
const hudMass = document.getElementById("hudMass");
const hudRank = document.getElementById("hudRank");
const hudAbilityName = document.getElementById("hudAbilityName");
const hudCooldownLabel = document.getElementById("hudCooldownLabel");
const hudCooldownFill = document.getElementById("hudCooldownFill");
const hudEffectLabel = document.getElementById("hudEffectLabel");
const hudEffectFill = document.getElementById("hudEffectFill");
const hudBuffRows = document.getElementById("hudBuffRows");
const hudBuffGrowthRow = document.querySelector('.hud-buff-row[data-buff="growth"]');
const hudBuffGrowthFill = document.getElementById("hudBuffGrowthFill");
const hudBuffSpeedRow = document.querySelector('.hud-buff-row[data-buff="speed"]');
const hudBuffSpeedFill = document.getElementById("hudBuffSpeedFill");
const hudBuffShieldRow = document.querySelector('.hud-buff-row[data-buff="shield"]');
const hudBuffShieldFill = document.getElementById("hudBuffShieldFill");
const hudControls = document.getElementById("hudControls");
const upgradePanel = document.getElementById("upgradePanel");
const upgradeToggle = document.getElementById("upgradeToggle");
const upgradeContent = document.getElementById("upgradeContent");
const upgradeCoins = document.getElementById("upgradeCoins");
const upgradeName = document.getElementById("upgradeName");
const upgradeDescription = document.getElementById("upgradeDescription");
const upgradeBuy = document.getElementById("upgradeBuy");

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

const UPGRADE_COST = 12;
const UPGRADE_DEFS = {
  classic: { name: "코어 과부하", description: "가속 에너지 탱크가 커져 더 오래 빠르게 달릴 수 있습니다." },
  blink: { name: "장거리 도약", description: "순간이동 거리가 더 길어집니다." },
  giant: { name: "돌진 거대화", description: "거대화 중에도 작은 적을 공격할 수 있습니다." },
  shield: { name: "강화 보호막", description: "보호막 지속 시간이 더 길어집니다." },
  magnet: { name: "중력 흡인", description: "10초 동안 작은 적까지 서서히 끌어당깁니다." },
  divider: { name: "즉시 재결합", description: "원할 때 언제든 다시 합쳐질 수 있습니다." },
};

const BENEFICIAL_EFFECT_META = {
  growth: {
    label: "유익균 성장",
    duration: 32000,
    color: "#8affcf",
    fillColor: "#8affcf",
    glow: "rgba(138, 255, 207, 0.28)",
  },
  speed: {
    label: "유익균 속도",
    duration: 18000,
    color: "#60b9ff",
    fillColor: "#9dd7ff",
    glow: "rgba(96, 185, 255, 0.28)",
  },
  shield: {
    label: "유익균 보호",
    duration: 10000,
    color: "#ffcf70",
    fillColor: "#ffe1a8",
    glow: "rgba(255, 207, 112, 0.28)",
  },
};

const world = {
  width: 3600,
  height: 3600,
};

const RESET_WARNING_WINDOW_MS = 5 * 60 * 1000;
const snapshotTextDecoder = new TextDecoder();
const COORD_QUANT_SCALE = 8;
const RADIUS_QUANT_SCALE = 8;
const VALUE_QUANT_SCALE = 16;
const SCALE_QUANT_SCALE = 1024;
const MASS_QUANT_SCALE = 16;
const DURATION_QUANT_STEP_MS = 100;

const state = {
  playerId: null,
  sessionId: null,
  nickname: "",
  playerMap: new Map(),
  players: [],
  leaderboard: [],
  chats: [],
  renderPlayers: new Map(),
  foodMap: new Map(),
  cactusMap: new Map(),
  leechMap: new Map(),
  wormholeMap: new Map(),
  foods: [],
  cacti: [],
  leeches: [],
  wormholes: [],
  mouse: { x: window.innerWidth / 2, y: window.innerHeight / 2 },
  camera: { x: 0, y: 0 },
  lastFrame: 0,
  connected: false,
  resetAt: 0,
  messageTimer: 0,
  pendingDirection: { x: 0, y: 0 },
  socket: null,
  inputTimer: null,
  selectedCellType: "classic",
  abilityPressed: false,
  splitPressed: false,
  mergePressed: false,
  upgradePanelOpen: false,
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
  chatCollapsed: true,
  chatLastActivityAt: 0,
  chatToasts: [],
  lastLeaderboardKey: "",
  lastChatRenderKey: "",
  chatComposing: false,
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
  strings: {
    ownerIds: new Map(),
    nicknames: new Map(),
    colors: new Map(),
    abilityNames: new Map(),
    cellTypes: new Map(),
  },
};

function isTypingInField() {
  const active = document.activeElement;
  return active === nicknameInput || active === chatInput;
}

if (state.isTouchDevice) {
  document.body.classList.add("touch-device");
  state.leaderboardCollapsed = true;
  state.minimapCollapsed = true;
  maybeShowFullscreenPrompt();
  updateRotatePrompt();
}

window.addEventListener("resize", () => {
  resizeCanvas();
  updateRotatePrompt();
});
window.addEventListener("orientationchange", () => {
  updateRotatePrompt();
  window.setTimeout(resizeCanvas, 60);
  window.setTimeout(resizeCanvas, 220);
  window.setTimeout(updateRotatePrompt, 60);
  window.setTimeout(updateRotatePrompt, 220);
});
window.visualViewport?.addEventListener("resize", () => {
  resizeCanvas();
  updateRotatePrompt();
});
document.addEventListener("fullscreenchange", updateRotatePrompt);
document.addEventListener("webkitfullscreenchange", updateRotatePrompt);
window.addEventListener("keydown", (event) => {
  const isChatTyping = document.activeElement === chatInput;
  if (isTypingInField() && !(event.code === "Enter" && isChatTyping)) {
    return;
  }
  if (event.code === "Space") { // 꾹 누를 때 연속 입력 허용
    state.abilityPressed = true;
  }
  if (event.code === "KeyW" && !event.repeat) {
    state.splitPressed = true;
  }
  if (event.code === "KeyE" && !event.repeat) {
    state.mergePressed = true;
  }
  if (event.code === "Enter" && state.connected) {
    if (isChatTyping) {
      if (event.isComposing || state.chatComposing) {
        return;
      }
      if (!chatInput.value.trim()) {
        event.preventDefault();
        setChatCollapsed(true);
        chatInput.blur();
      }
      return;
    }
    event.preventDefault();
    if (state.chatCollapsed) {
      setChatCollapsed(false);
    }
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
touchMerge.addEventListener("pointerdown", (event) => {
  event.preventDefault();
  state.mergePressed = true;
});
touchMerge.textContent = "합체";
touchSplit.textContent = "W";
fullscreenAccept.addEventListener("click", async () => {
  hideFullscreenPrompt();
  const entered = await requestMobileFullscreen();
  if (!entered) {
    showMessage("브라우저에서 전체화면 전환이 제한되었습니다.");
  }
  await lockLandscapeOrientation();
  updateRotatePrompt();
});
fullscreenDismiss.addEventListener("click", () => {
  hideFullscreenPrompt();
});
bindTouchPanelToggle(minimapToggle, toggleMinimapPanel);
bindTouchPanelToggle(leaderboardToggle, toggleLeaderboardPanel);
upgradeBuy.addEventListener("click", () => {
  sendUpgradePurchase();
});
upgradeToggle.addEventListener("click", () => {
  state.upgradePanelOpen = !state.upgradePanelOpen;
  syncUpgradePanelState();
});
bindTouchPanelToggle(chatToggle, toggleChatPanel);

chatForm.addEventListener("submit", (event) => {
  event.preventDefault();
  if (state.chatComposing) {
    return;
  }
  sendChat();
});

chatInput.addEventListener("compositionstart", () => {
  state.chatComposing = true;
});

chatInput.addEventListener("compositionend", () => {
  state.chatComposing = false;
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
    if (state.isTouchDevice && !isFullscreenActive()) {
      maybeShowFullscreenPrompt();
    }
    connectSocket();
  } catch {
    showMessage("서버 연결에 실패했습니다.");
  }
});

function resizeCanvas() {
  const viewportWidth = window.visualViewport?.width || window.innerWidth;
  const viewportHeight = window.visualViewport?.height || window.innerHeight;
  canvas.width = Math.max(1, Math.round(viewportWidth));
  canvas.height = Math.max(1, Math.round(viewportHeight));
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
  socket.binaryType = "arraybuffer";
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
    if (typeof event.data !== "string") {
      handleBinarySnapshot(event.data);
      return;
    }

    const data = JSON.parse(event.data);
    if (data.type === "snapshot") {
      const snapshotAt = performance.now();
      const previousMe = state.renderPlayers.get(state.playerId);
      if (state.lastSnapshotAt > 0) {
        state.snapshotGap = Math.max(16, Math.min(140, snapshotAt - state.lastSnapshotAt));
      }
      state.lastSnapshotAt = snapshotAt;
      const nextPlayers = data.players || [];
      state.players = nextPlayers;
      applySnapshotObjects(data);
      syncRenderPlayers(nextPlayers, snapshotAt);
      const me = state.renderPlayers.get(state.playerId);
      maybeShowBeneficialEffectMessage(previousMe, me);
      const grouped = state.leaderboard.length > 0 ? state.leaderboard : aggregateOwners(nextPlayers);
      const myOwnerId = me ? (me.ownerId || me.id) : state.playerId;
      const myGroup = grouped.find((entry) => entry.ownerId === myOwnerId);
      if (me) {
        hudMass.textContent = isRespawningPlayer(me) ? `부활 ${Math.max(1, Math.ceil((me.respawnRemaining || 0) / 1000))}초` : Math.round(myGroup ? myGroup.mass : effectiveCombatMassClient(me));
        updateAbilityHud(me);
      }
      renderLeaderboard();
      renderChat();
      return;
    }

    if (data.type === "leaderboard") {
      state.leaderboard = data.leaderboard || [];
      renderLeaderboard();
      return;
    }

    if (data.type === "chats") {
      const nextChats = data.chats || [];
      handleIncomingChats(nextChats);
      state.chats = nextChats;
      renderChat();
      return;
    }

    if (data.type === "config") {
      applyRuntimeConfig(data.config);
      return;
    }

    if (data.type === "reset") {
      state.resetAt = Number(data.resetAt) || 0;
      return;
    }

    if (data.type === "stringTable") {
      applyStringTableUpdate(data);
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
  } else if (state.isTouchDevice) {
    state.pendingDirection.x = 0;
    state.pendingDirection.y = 0;
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
    useMerge: state.mergePressed,
  }));
  state.splitPressed = false;
  state.mergePressed = false;
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

function sendUpgradePurchase() {
  const me = getOwnedRepresentativePlayer();
  if (!me || !state.connected || !state.socket || state.socket.readyState !== WebSocket.OPEN) {
    return;
  }
  state.socket.send(JSON.stringify({
    type: "upgrade",
    upgrade: me.cellType,
  }));
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
  renderResetNotice();
  renderUpgradePanel();
  renderControlHints();

  if (state.messageTimer > 0) {
    state.messageTimer -= dt;
    if (state.messageTimer <= 0) {
      messageBox.classList.add("hidden");
    }
  }
  if (pruneChatToasts(timestamp)) {
    renderChatToasts();
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
      existing.shieldRemaining = player.shieldRemaining || 0;
      existing.probioticRemaining = player.probioticRemaining || 0;
      existing.speedBoostRemaining = player.speedBoostRemaining || 0;
      existing.respawnRemaining = player.respawnRemaining || 0;
      existing.scale = player.scale;
      existing.isBot = player.isBot;
      existing.coins = player.coins || 0;
      existing.upgrades = player.upgrades || {};
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
        coins: player.coins || 0,
        upgrades: player.upgrades || {},
        shieldRemaining: player.shieldRemaining || 0,
        probioticRemaining: player.probioticRemaining || 0,
        speedBoostRemaining: player.speedBoostRemaining || 0,
        respawnRemaining: player.respawnRemaining || 0,
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
  const metrics = getOwnedMetricsFromRenderPlayers();
  if (!metrics) {
    return;
  }
  state.camera.x = lerp(state.camera.x, metrics.x, 0.16);
  state.camera.y = lerp(state.camera.y, metrics.y, 0.16);
}

function render() {
  ctx.clearRect(0, 0, canvas.width, canvas.height);
  ctx.fillStyle = "#08101d";
  ctx.fillRect(0, 0, canvas.width, canvas.height);
  ctx.save();
  ctx.translate(canvas.width / 2, canvas.height / 2);
  ctx.scale(state.zoom, state.zoom);
  ctx.translate(-canvas.width / 2, -canvas.height / 2);
  drawBackground();
  drawWormholes();
  drawCacti();
  drawFoods();
  drawPlayers();
  drawLeeches();
  ctx.restore();
  drawMinimap();
}

function getVisibleWorldBounds(padding = 0) {
  const zoom = Math.max(state.zoom, 0.01);
  const halfWidth = canvas.width / zoom * 0.5 + padding;
  const halfHeight = canvas.height / zoom * 0.5 + padding;
  return {
    left: state.camera.x - halfWidth,
    right: state.camera.x + halfWidth,
    top: state.camera.y - halfHeight,
    bottom: state.camera.y + halfHeight,
  };
}

function isCircleInView(x, y, radius, bounds) {
  return !(x + radius < bounds.left || x - radius > bounds.right || y + radius < bounds.top || y - radius > bounds.bottom);
}

function drawBackground() {
  ctx.fillStyle = "#08101d";
  ctx.fillRect(0, 0, canvas.width, canvas.height);

  const bounds = getVisibleWorldBounds(140);
  const gridSize = 120;
  const startX = Math.max(0, Math.floor(bounds.left / gridSize) * gridSize);
  const endX = Math.min(world.width, Math.ceil(bounds.right / gridSize) * gridSize);
  const startY = Math.max(0, Math.floor(bounds.top / gridSize) * gridSize);
  const endY = Math.min(world.height, Math.ceil(bounds.bottom / gridSize) * gridSize);

  ctx.save();
  ctx.translate(canvas.width / 2 - state.camera.x, canvas.height / 2 - state.camera.y);
  ctx.strokeStyle = "rgba(255,255,255,0.05)";
  for (let x = startX; x <= endX; x += gridSize) {
    ctx.beginPath();
    ctx.moveTo(x, startY);
    ctx.lineTo(x, endY);
    ctx.stroke();
  }
  for (let y = startY; y <= endY; y += gridSize) {
    ctx.beginPath();
    ctx.moveTo(startX, y);
    ctx.lineTo(endX, y);
    ctx.stroke();
  }
  ctx.restore();
}

function drawFoods() {
  const bounds = getVisibleWorldBounds(90);
  ctx.save();
  ctx.translate(canvas.width / 2 - state.camera.x, canvas.height / 2 - state.camera.y);
  for (const food of state.foods) {
    if (!isCircleInView(food.x, food.y, food.radius + 8, bounds)) {
      continue;
    }
    if (food.kind === "probiotic" || food.kind === "probiotic-growth" || food.kind === "probiotic-speed" || food.kind === "probiotic-shield") {
      const pulse = 1 + Math.sin(state.time * 3.4 + food.x * 0.01) * 0.08;
      const radius = food.radius * pulse;
      const palette = getBeneficialFoodPalette(food.kind);
      const gradient = ctx.createRadialGradient(
        food.x - radius * 0.25,
        food.y - radius * 0.3,
        radius * 0.15,
        food.x,
        food.y,
        radius * 1.15,
      );
      gradient.addColorStop(0, palette.core);
      gradient.addColorStop(0.4, palette.mid);
      gradient.addColorStop(1, palette.outer);
      ctx.fillStyle = gradient;
      ctx.beginPath();
      ctx.arc(food.x, food.y, radius, 0, Math.PI * 2);
      ctx.fill();

      ctx.strokeStyle = palette.ring;
      ctx.lineWidth = 2;
      ctx.stroke();

      ctx.strokeStyle = palette.halo;
      ctx.lineWidth = 1.2;
      ctx.beginPath();
      ctx.arc(food.x, food.y, radius + 4 + Math.sin(state.time * 2.2 + food.y * 0.01) * 1.5, 0, Math.PI * 2);
      ctx.stroke();

      drawBeneficialFoodIcon(food, radius, palette);
      continue;
    }

    ctx.fillStyle = "#8affcf";
    ctx.beginPath();
    ctx.arc(food.x, food.y, food.radius, 0, Math.PI * 2);
    ctx.fill();
  }
  ctx.restore();
}

function getBeneficialFoodPalette(kind) {
  switch (kind) {
    case "probiotic-speed":
      return {
        core: "#eefbff",
        mid: "#7ce7ff",
        outer: "#2aa7e3",
        ring: "rgba(214,247,255,0.92)",
        halo: "rgba(124,231,255,0.34)",
        icon: "rgba(233,250,255,0.96)",
        iconFill: "rgba(233,250,255,0.9)",
      };
    case "probiotic-shield":
      return {
        core: "#fff7db",
        mid: "#ffd86a",
        outer: "#ff9b2f",
        ring: "rgba(255,244,209,0.95)",
        halo: "rgba(255,204,92,0.34)",
        icon: "rgba(255,248,227,0.96)",
        iconFill: "rgba(255,238,184,0.88)",
      };
    default:
      return {
        core: "#f9fff3",
        mid: "#b9ff84",
        outer: "#4fcf62",
        ring: "rgba(240,255,220,0.9)",
        halo: "rgba(185,255,132,0.35)",
        icon: "rgba(243,255,232,0.95)",
        iconFill: "rgba(227,255,202,0.9)",
      };
  }
}

function drawBeneficialFoodIcon(food, radius, palette) {
  ctx.save();
  ctx.translate(food.x, food.y);
  ctx.strokeStyle = palette.icon;
  ctx.fillStyle = palette.iconFill;
  ctx.lineWidth = Math.max(1.8, radius * 0.14);
  ctx.lineCap = "round";
  ctx.lineJoin = "round";

  if (food.kind === "probiotic-speed") {
    ctx.rotate(Math.sin(state.time * 2.6 + food.x * 0.01) * 0.08);
    ctx.beginPath();
    ctx.moveTo(-radius * 0.28, -radius * 0.16);
    ctx.lineTo(radius * 0.02, -radius * 0.16);
    ctx.lineTo(-radius * 0.08, radius * 0.22);
    ctx.lineTo(radius * 0.3, radius * 0.02);
    ctx.stroke();
    ctx.beginPath();
    ctx.arc(-radius * 0.18, radius * 0.12, radius * 0.08, 0, Math.PI * 2);
    ctx.fill();
    ctx.beginPath();
    ctx.arc(radius * 0.04, radius * 0.06, radius * 0.08, 0, Math.PI * 2);
    ctx.fill();
    ctx.beginPath();
    ctx.arc(radius * 0.22, -radius * 0.02, radius * 0.08, 0, Math.PI * 2);
    ctx.fill();
    ctx.restore();
    return;
  }

  if (food.kind === "probiotic-shield") {
    ctx.beginPath();
    ctx.moveTo(0, -radius * 0.34);
    ctx.bezierCurveTo(radius * 0.28, -radius * 0.3, radius * 0.38, -radius * 0.04, radius * 0.28, radius * 0.24);
    ctx.bezierCurveTo(radius * 0.18, radius * 0.44, radius * 0.02, radius * 0.54, 0, radius * 0.58);
    ctx.bezierCurveTo(-radius * 0.02, radius * 0.54, -radius * 0.18, radius * 0.44, -radius * 0.28, radius * 0.24);
    ctx.bezierCurveTo(-radius * 0.38, -radius * 0.04, -radius * 0.28, -radius * 0.3, 0, -radius * 0.34);
    ctx.closePath();
    ctx.fill();
    ctx.stroke();
    ctx.beginPath();
    ctx.moveTo(0, -radius * 0.18);
    ctx.lineTo(0, radius * 0.28);
    ctx.moveTo(-radius * 0.18, 0);
    ctx.lineTo(radius * 0.18, 0);
    ctx.stroke();
    ctx.restore();
    return;
  }

  ctx.rotate(Math.sin(state.time * 1.8 + food.y * 0.01) * 0.12);
  for (let i = 0; i < 6; i += 1) {
    const angle = (Math.PI * 2 * i) / 6;
    const x = Math.cos(angle) * radius * 0.3;
    const y = Math.sin(angle) * radius * 0.3;
    ctx.beginPath();
    ctx.arc(x, y, radius * 0.12, 0, Math.PI * 2);
    ctx.fill();
    ctx.beginPath();
    ctx.moveTo(0, 0);
    ctx.lineTo(x, y);
    ctx.stroke();
  }
  ctx.beginPath();
  ctx.arc(0, 0, radius * 0.11, 0, Math.PI * 2);
  ctx.fill();
  ctx.restore();
}

function drawWormholes() {
  const bounds = getVisibleWorldBounds(180);
  ctx.save();
  ctx.translate(canvas.width / 2 - state.camera.x, canvas.height / 2 - state.camera.y);

  for (const hole of state.wormholes) {
    if (!isCircleInView(hole.x, hole.y, hole.pullRange, bounds)) {
      continue;
    }
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
  const bounds = getVisibleWorldBounds(140);
  ctx.save();
  ctx.translate(canvas.width / 2 - state.camera.x, canvas.height / 2 - state.camera.y);

  for (const cactus of state.cacti) {
    if (!isCircleInView(cactus.x, cactus.y, cactus.size * 1.5, bounds)) {
      continue;
    }
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

function drawLeeches() {
  const bounds = getVisibleWorldBounds(160);
  ctx.save();
  ctx.translate(canvas.width / 2 - state.camera.x, canvas.height / 2 - state.camera.y);

  for (const leech of state.leeches) {
    const size = leech.size || 18;
    if (!isCircleInView(leech.x, leech.y, size * 2.4, bounds)) {
      continue;
    }
    const phase = state.time * 5.8 + hashString(leech.id || "leech") * 0.00001;
    if ((leech.burstRemaining || 0) > 0) {
      drawLeechBurst(leech, size, phase);
      continue;
    }
    const attached = Boolean(leech.attachedTo);
    if (attached) {
      const target = state.renderPlayers.get(leech.attachedTo);
      if (target && !isRespawningPlayer(target)) {
        drawAttachedLeech(leech, target, size, phase);
        continue;
      }
    }
    const wiggle = Math.sin(phase) * (attached ? 0.04 : 0.12);
    const angle = (leech.angle || 0) + wiggle;
    const bodyLength = size * (attached ? 2.35 : 2.75);
    const bodyWidth = size * (attached ? 0.72 : 0.62);

    ctx.save();
    ctx.translate(leech.x, leech.y);
    ctx.rotate(angle);

    ctx.shadowColor = attached ? "rgba(255, 71, 94, 0.42)" : "rgba(4, 8, 14, 0.55)";
    ctx.shadowBlur = attached ? 13 : 8;

    const gradient = ctx.createRadialGradient(
      -bodyLength * 0.24,
      -bodyWidth * 0.3,
      bodyWidth * 0.08,
      0,
      0,
      bodyLength * 0.62,
    );
    gradient.addColorStop(0, attached ? "rgba(255, 132, 143, 0.96)" : "rgba(144, 93, 150, 0.95)");
    gradient.addColorStop(0.48, attached ? "rgba(118, 28, 52, 0.98)" : "rgba(68, 38, 86, 0.98)");
    gradient.addColorStop(1, attached ? "rgba(33, 10, 23, 0.98)" : "rgba(21, 15, 34, 0.98)");

    ctx.fillStyle = gradient;
    ctx.beginPath();
    ctx.ellipse(0, 0, bodyLength * 0.5, bodyWidth * 0.5, 0, 0, Math.PI * 2);
    ctx.fill();

    ctx.strokeStyle = attached ? "rgba(255, 154, 163, 0.6)" : "rgba(185, 150, 216, 0.42)";
    ctx.lineWidth = Math.max(1, size * 0.08);
    ctx.beginPath();
    ctx.ellipse(0, 0, bodyLength * 0.5, bodyWidth * 0.5, 0, 0, Math.PI * 2);
    ctx.stroke();

    ctx.shadowBlur = 0;
    ctx.strokeStyle = attached ? "rgba(255, 207, 213, 0.24)" : "rgba(229, 210, 255, 0.2)";
    ctx.lineWidth = Math.max(1, size * 0.035);
    for (let i = -2; i <= 2; i += 1) {
      const x = i * bodyLength * 0.14;
      ctx.beginPath();
      ctx.ellipse(x, 0, bodyWidth * 0.22, bodyWidth * 0.48, 0.12 * i, Math.PI * 0.5, Math.PI * 1.5);
      ctx.stroke();
    }

    ctx.fillStyle = attached ? "rgba(255, 47, 75, 0.86)" : "rgba(230, 86, 121, 0.72)";
    ctx.beginPath();
    ctx.ellipse(bodyLength * 0.42, 0, bodyWidth * 0.25, bodyWidth * 0.36, 0, 0, Math.PI * 2);
    ctx.fill();

    ctx.strokeStyle = attached ? "rgba(255, 230, 234, 0.8)" : "rgba(255, 214, 224, 0.45)";
    ctx.lineWidth = Math.max(1, size * 0.05);
    ctx.beginPath();
    ctx.ellipse(bodyLength * 0.44, 0, bodyWidth * 0.13, bodyWidth * 0.2, 0, 0, Math.PI * 2);
    ctx.stroke();

    ctx.fillStyle = "rgba(255,255,255,0.42)";
    ctx.beginPath();
    ctx.ellipse(-bodyLength * 0.16, -bodyWidth * 0.18, bodyLength * 0.16, bodyWidth * 0.09, -0.2, 0, Math.PI * 2);
    ctx.fill();

    if (attached) {
      const pulse = 1 + Math.sin(phase * 1.8) * 0.08;
      ctx.strokeStyle = "rgba(255, 84, 103, 0.48)";
      ctx.lineWidth = Math.max(1, size * 0.08);
      ctx.beginPath();
      ctx.arc(bodyLength * 0.45, 0, bodyWidth * 0.38 * pulse, 0, Math.PI * 2);
      ctx.stroke();
    }

    ctx.restore();
  }

  ctx.restore();
}

function drawAttachedLeech(leech, target, size, phase) {
  const targetRadius = target.drawRadius || target.radius || 28;
  const angle = Number.isFinite(leech.angle) ? leech.angle - Math.PI / 2 : Math.atan2(leech.y - target.drawY, leech.x - target.drawX);
  const wrapRadius = Math.max(targetRadius * 0.78, targetRadius - size * 0.1);
  const span = clampRange(0.7 + size / Math.max(80, targetRadius * 4), 0.62, 1.18);
  const start = angle - span * 0.5;
  const end = angle + span * 0.5;
  const midX = target.drawX + Math.cos(angle) * wrapRadius;
  const midY = target.drawY + Math.sin(angle) * wrapRadius;
  const pulse = 1 + Math.sin(phase * 1.8) * 0.06;

  ctx.save();
  ctx.lineCap = "round";
  ctx.lineJoin = "round";
  ctx.shadowColor = "rgba(255, 54, 80, 0.42)";
  ctx.shadowBlur = 14;

  ctx.strokeStyle = "rgba(44, 8, 20, 0.96)";
  ctx.lineWidth = Math.max(6, size * 0.78 * pulse);
  ctx.beginPath();
  ctx.arc(target.drawX, target.drawY, wrapRadius, start, end);
  ctx.stroke();

  ctx.shadowBlur = 0;
  ctx.strokeStyle = "rgba(132, 26, 54, 0.98)";
  ctx.lineWidth = Math.max(5, size * 0.58 * pulse);
  ctx.beginPath();
  ctx.arc(target.drawX, target.drawY, wrapRadius, start, end);
  ctx.stroke();

  ctx.strokeStyle = "rgba(255, 128, 143, 0.62)";
  ctx.lineWidth = Math.max(2, size * 0.16);
  ctx.beginPath();
  ctx.arc(target.drawX, target.drawY, wrapRadius - size * 0.1, start + 0.08, end - 0.08);
  ctx.stroke();

  for (let i = 0; i < 5; i += 1) {
    const t = i / 4;
    const ribAngle = lerp(start, end, t);
    const x = target.drawX + Math.cos(ribAngle) * wrapRadius;
    const y = target.drawY + Math.sin(ribAngle) * wrapRadius;
    ctx.save();
    ctx.translate(x, y);
    ctx.rotate(ribAngle + Math.PI / 2);
    ctx.strokeStyle = "rgba(255, 205, 214, 0.22)";
    ctx.lineWidth = Math.max(1, size * 0.055);
    ctx.beginPath();
    ctx.ellipse(0, 0, size * 0.18, size * 0.42, 0, Math.PI * 0.5, Math.PI * 1.5);
    ctx.stroke();
    ctx.restore();
  }

  ctx.fillStyle = "rgba(255, 44, 73, 0.92)";
  ctx.beginPath();
  ctx.ellipse(midX, midY, size * 0.38, size * 0.5, angle + Math.PI / 2, 0, Math.PI * 2);
  ctx.fill();

  ctx.strokeStyle = "rgba(255, 230, 234, 0.8)";
  ctx.lineWidth = Math.max(1, size * 0.06);
  ctx.beginPath();
  ctx.ellipse(midX, midY, size * 0.18, size * 0.28, angle + Math.PI / 2, 0, Math.PI * 2);
  ctx.stroke();

  ctx.strokeStyle = "rgba(255, 64, 90, 0.42)";
  ctx.lineWidth = Math.max(1, size * 0.08);
  ctx.beginPath();
  ctx.arc(midX, midY, size * 0.48 * pulse, 0, Math.PI * 2);
  ctx.stroke();

  ctx.restore();
}

function drawLeechBurst(leech, size, phase) {
  const remaining = clamp01((leech.burstRemaining || 0) / 1200);
  const progress = 1 - remaining;
  const seed = hashString(leech.id || "burst");

  ctx.save();
  ctx.translate(leech.x, leech.y);
  ctx.globalAlpha = remaining;

  ctx.strokeStyle = "rgba(255, 82, 105, 0.72)";
  ctx.lineWidth = Math.max(1, size * 0.08);
  ctx.beginPath();
  ctx.arc(0, 0, size * (0.7 + progress * 1.7), 0, Math.PI * 2);
  ctx.stroke();

  for (let i = 0; i < 14; i += 1) {
    const angle = (Math.PI * 2 * i) / 14 + seededUnit(seed, i) * 0.32;
    const distance = size * (0.35 + progress * (1.6 + seededUnit(seed, i + 17) * 0.7));
    const shardSize = size * (0.08 + seededUnit(seed, i + 31) * 0.11);
    ctx.fillStyle = i % 3 === 0 ? "rgba(255, 210, 180, 0.82)" : "rgba(190, 28, 62, 0.86)";
    ctx.beginPath();
    ctx.ellipse(
      Math.cos(angle) * distance,
      Math.sin(angle) * distance,
      shardSize * 1.5,
      shardSize,
      angle + phase * 0.15,
      0,
      Math.PI * 2,
    );
    ctx.fill();
  }

  ctx.fillStyle = "rgba(70, 8, 24, 0.78)";
  ctx.beginPath();
  ctx.arc(0, 0, size * Math.max(0.2, 0.7 - progress * 0.45), 0, Math.PI * 2);
  ctx.fill();
  ctx.restore();
}

function drawPlayers() {
  const me = state.renderPlayers.get(state.playerId);
  const bounds = getVisibleWorldBounds(120);
  ctx.save();
  ctx.translate(canvas.width / 2 - state.camera.x, canvas.height / 2 - state.camera.y);

  for (const player of state.renderPlayers.values()) {
    if (isRespawningPlayer(player)) {
      continue;
    }
    if (!isCircleInView(player.drawX, player.drawY, player.drawRadius + 14, bounds)) {
      continue;
    }
    const isMe = player.id === state.playerId;
    const isSameOwner = me && (me.ownerId || me.id) === (player.ownerId || player.id);
    const canEat = me && !isSameOwner && me.id !== player.id && canPlayerEatClient(me, player);
    const canBeEaten = me && !isSameOwner && me.id !== player.id && canPlayerEatClient(player, me);
    const deformation = getFragmentDeformation(player);

    ctx.save();
    ctx.translate(player.drawX, player.drawY);
    ctx.rotate(deformation.angle);
    ctx.scale(deformation.scaleX, deformation.scaleY);
    drawDropletPlayerBody(player, {
      isMe,
      isSameOwner,
      canEat,
      canBeEaten,
      deformation,
    });
    ctx.restore();

    if (player.effectRemaining > 0) {
      ctx.strokeStyle = "rgba(255, 205, 112, 0.9)";
      ctx.lineWidth = 3;
      ctx.beginPath();
      ctx.arc(player.drawX, player.drawY, player.drawRadius + 8, 0, Math.PI * 2);
      ctx.stroke();
    }

    const beneficialEffects = getBeneficialEffectEntries(player);
    if (beneficialEffects.length > 0) {
      drawBeneficialEffectRings(player, beneficialEffects);
    }

    drawPlayerLabel(player);
  }

  ctx.restore();
}

function drawPlayerLabel(player) {
  const nameFontSize = Math.max(12, Math.min(player.drawRadius * 0.58, player.drawRadius * (player.nickname.length <= 4 ? 0.7 : 0.56)));
  const massFontSize = Math.max(10, Math.min(player.drawRadius * 0.3, 24));
  const nameY = player.drawY - Math.min(10, player.drawRadius * 0.14);
  const massY = player.drawY + Math.min(16, player.drawRadius * 0.22);

  ctx.save();
  ctx.textAlign = "center";
  ctx.textBaseline = "middle";
  ctx.shadowColor = "rgba(2, 8, 18, 0.85)";
  ctx.shadowBlur = 7;
  ctx.lineWidth = 3;
  ctx.strokeStyle = "rgba(2, 8, 18, 0.58)";
  ctx.fillStyle = "#eef7ff";
  ctx.font = `700 ${nameFontSize}px Segoe UI`;
  ctx.strokeText(player.nickname, player.drawX, nameY);
  ctx.fillText(player.nickname, player.drawX, nameY);
  ctx.font = `${massFontSize}px Segoe UI`;
  ctx.fillStyle = "rgba(238,247,255,0.82)";
  const massText = String(Math.round(effectiveCombatMassClient(player)));
  ctx.strokeText(massText, player.drawX, massY);
  ctx.fillText(massText, player.drawX, massY);
  ctx.restore();
}

function drawDropletPlayerBody(player, flags) {
  const radius = player.drawRadius;
  const screenRadius = radius * state.zoom;
  const quality = getDropletRenderQuality(radius, screenRadius);
  const seed = hashString(player.id || player.nickname || player.color || "cell");
  const palette = getDropletPalette(player.color, seed);
  const stretchFix = 1 / Math.max(0.78, flags.deformation.scaleX);

  if (quality === "simple") {
    ctx.fillStyle = player.color;
    ctx.beginPath();
    ctx.arc(0, 0, radius, 0, Math.PI * 2);
    ctx.fill();
    drawDropletOutline(radius, flags, stretchFix);
    return;
  }

  const phase = state.time * 2.8 + player.drawX * 0.009 + player.drawY * 0.007;
  const pulse = Math.sin(phase) * 0.018;
  const wobbleScale = quality === "rich" ? 1 + pulse : 1;

  ctx.save();
  ctx.scale(wobbleScale, 1 - pulse * 0.45);

  const body = ctx.createRadialGradient(
    -radius * 0.26,
    -radius * 0.32,
    radius * 0.08,
    radius * 0.08,
    radius * 0.12,
    radius * 1.08,
  );
  body.addColorStop(0, palette.membraneLight);
  body.addColorStop(0.28, palette.cytoplasmLight);
  body.addColorStop(0.52, palette.cytoplasm);
  body.addColorStop(0.78, palette.cytoplasmDeep);
  body.addColorStop(1, palette.membraneEdge);

  ctx.fillStyle = body;
  ctx.beginPath();
  ctx.arc(0, 0, radius, 0, Math.PI * 2);
  ctx.fill();

  if (quality !== "simple") {
    drawCellInterior(radius, palette, phase, quality, seed);
  }

  drawDropletGlass(radius, palette, quality, phase, seed);
  drawDropletOutline(radius, flags, stretchFix);
  ctx.restore();
}

function getDropletRenderQuality(radius, screenRadius) {
  if (screenRadius < 9 || radius < 10) {
    return "simple";
  }
  if (screenRadius < 22 || radius < 22 || state.renderPlayers.size > 80) {
    return "medium";
  }
  return "rich";
}

function drawDropletGlass(radius, palette, quality, phase, seed) {
  const rim = ctx.createRadialGradient(
    radius * 0.18,
    radius * 0.24,
    radius * 0.42,
    0,
    0,
    radius * 1.03,
  );
  rim.addColorStop(0, "rgba(255,255,255,0)");
  rim.addColorStop(0.62, "rgba(255,255,255,0)");
  rim.addColorStop(0.86, palette.rim);
  rim.addColorStop(1, palette.membraneStroke);
  ctx.fillStyle = rim;
  ctx.beginPath();
  ctx.arc(0, 0, radius, 0, Math.PI * 2);
  ctx.fill();

  if (quality === "rich") {
    drawMembraneTexture(radius, palette, phase, seed);
  }

  ctx.fillStyle = quality === "rich" ? "rgba(235,247,255,0.3)" : "rgba(235,247,255,0.2)";
  ctx.beginPath();
  ctx.ellipse(
    -radius * 0.28 + Math.sin(phase * 1.2) * radius * 0.025,
    -radius * 0.34 + Math.cos(phase) * radius * 0.018,
    radius * 0.2,
    radius * 0.09,
    -0.55,
    0,
    Math.PI * 2,
  );
  ctx.fill();

  if (quality !== "rich") {
    return;
  }

  ctx.strokeStyle = "rgba(210,232,255,0.32)";
  ctx.lineWidth = Math.max(1, radius * 0.03);
  ctx.lineCap = "round";
  ctx.beginPath();
  ctx.arc(-radius * 0.05, -radius * 0.04, radius * 0.62, Math.PI * 1.1, Math.PI * 1.55);
  ctx.stroke();
}

function drawMembraneTexture(radius, palette, phase, seed) {
  ctx.save();
  ctx.lineCap = "round";
  ctx.strokeStyle = palette.membraneBand;
  ctx.lineWidth = Math.max(1, radius * 0.018);
  for (let i = 0; i < 3; i += 1) {
    const start = Math.PI * (0.08 + seededUnit(seed, i + 151) * 0.46) + Math.sin(phase * 0.35 + i) * 0.08;
    const end = start + Math.PI * (0.36 + seededUnit(seed, i + 161) * 0.18);
    const bandRadius = radius * (0.66 + i * 0.09);
    ctx.beginPath();
    ctx.arc(0, 0, bandRadius, start, end);
    ctx.stroke();
  }

  ctx.fillStyle = palette.membranePore;
  for (let i = 0; i < 12; i += 1) {
    const angle = seededUnit(seed, i + 171) * Math.PI * 2 + phase * 0.025;
    const distance = radius * (0.72 + seededUnit(seed, i + 181) * 0.2);
    const x = Math.cos(angle) * distance;
    const y = Math.sin(angle) * distance;
    ctx.beginPath();
    ctx.arc(x, y, radius * (0.008 + seededUnit(seed, i + 191) * 0.01), 0, Math.PI * 2);
    ctx.fill();
  }
  ctx.restore();
}

function drawCellInterior(radius, palette, phase, quality, seed) {
  drawCellNucleus(radius, palette, phase, seed, quality);

  ctx.save();
  ctx.globalAlpha = quality === "rich" ? 0.42 : 0.24;

  if (quality === "rich") {
    ctx.strokeStyle = palette.strand;
    ctx.lineWidth = Math.max(1, radius * 0.012);
    ctx.lineCap = "round";
    for (let i = 0; i < 5; i += 1) {
      const angle = phase * 0.22 + i * 1.9 + seededUnit(seed, i) * 0.7;
      const x = Math.cos(angle) * radius * (0.16 + seededUnit(seed, i + 9) * 0.3);
      const y = Math.sin(angle) * radius * (0.14 + seededUnit(seed, i + 17) * 0.28);
      ctx.beginPath();
      ctx.moveTo(x - radius * 0.14, y + Math.sin(phase + i) * radius * 0.03);
      ctx.bezierCurveTo(
        x - radius * 0.02,
        y - radius * 0.08,
        x + radius * 0.08,
        y + radius * 0.07,
        x + radius * 0.18,
        y - Math.cos(phase + i) * radius * 0.025,
      );
      ctx.stroke();
    }

    for (let i = 0; i < 7; i += 1) {
      const angle = seededUnit(seed, i + 211) * Math.PI * 2 + phase * (0.018 + seededUnit(seed, i + 221) * 0.025);
      const distance = radius * Math.sqrt(seededUnit(seed, i + 231)) * 0.58;
      const x = Math.cos(angle) * distance;
      const y = Math.sin(angle * 1.09) * distance * 0.85;
      const vesicleRadius = radius * (0.035 + seededUnit(seed, i + 241) * 0.045);
      ctx.fillStyle = palette.vesicle;
      ctx.beginPath();
      ctx.arc(x, y, vesicleRadius, 0, Math.PI * 2);
      ctx.fill();
      ctx.strokeStyle = palette.vesicleRing;
      ctx.lineWidth = Math.max(1, radius * 0.008);
      ctx.beginPath();
      ctx.arc(x, y, vesicleRadius, 0, Math.PI * 2);
      ctx.stroke();
    }
  }

  const granuleCount = quality === "rich" ? 74 : 22;
  ctx.lineCap = "round";
  for (let i = 0; i < granuleCount; i += 1) {
    const angle = seededUnit(seed, i + 31) * Math.PI * 2 + phase * (0.025 + seededUnit(seed, i + 41) * 0.035);
    const distance = radius * Math.sqrt(seededUnit(seed, i + 53)) * 0.78;
    const x = Math.cos(angle) * distance;
    const y = Math.sin(angle * 1.11) * distance * 0.86;
    const dotRadius = radius * (0.006 + seededUnit(seed, i + 67) * (quality === "rich" ? 0.018 : 0.012));
    ctx.fillStyle = i % 7 === 0 ? palette.granuleLight : i % 4 === 0 ? palette.granuleDark : palette.granule;
    ctx.beginPath();
    ctx.arc(x, y, dotRadius, 0, Math.PI * 2);
    ctx.fill();
  }
  ctx.restore();
}

function drawCellNucleus(radius, palette, phase, seed, quality) {
  const nucleusRadius = radius * (quality === "rich" ? 0.27 : 0.23);
  const x = radius * (-0.03 + (seededUnit(seed, 3) - 0.5) * 0.18) + Math.sin(phase * 0.55) * radius * 0.018;
  const y = radius * (0.04 + (seededUnit(seed, 5) - 0.5) * 0.16) + Math.cos(phase * 0.5) * radius * 0.014;
  const nucleus = ctx.createRadialGradient(
    x - nucleusRadius * 0.28,
    y - nucleusRadius * 0.34,
    nucleusRadius * 0.12,
    x,
    y,
    nucleusRadius,
  );
  nucleus.addColorStop(0, palette.nucleusLight);
  nucleus.addColorStop(0.58, palette.nucleus);
  nucleus.addColorStop(1, palette.nucleusEdge);

  ctx.fillStyle = nucleus;
  ctx.beginPath();
  ctx.ellipse(x, y, nucleusRadius * 1.04, nucleusRadius * 0.92, Math.sin(phase) * 0.2, 0, Math.PI * 2);
  ctx.fill();

  ctx.strokeStyle = palette.nucleusRing;
  ctx.lineWidth = Math.max(1, radius * 0.02);
  ctx.beginPath();
  ctx.ellipse(x, y, nucleusRadius * 1.04, nucleusRadius * 0.92, Math.sin(phase) * 0.2, 0, Math.PI * 2);
  ctx.stroke();

  if (quality !== "rich") {
    return;
  }

  ctx.fillStyle = palette.nucleolus;
  ctx.beginPath();
  ctx.arc(x + nucleusRadius * 0.16, y - nucleusRadius * 0.12, nucleusRadius * 0.2, 0, Math.PI * 2);
  ctx.fill();

  ctx.fillStyle = palette.chromatin;
  for (let i = 0; i < 7; i += 1) {
    const angle = seededUnit(seed, i + 271) * Math.PI * 2 + phase * 0.035;
    const distance = nucleusRadius * Math.sqrt(seededUnit(seed, i + 281)) * 0.62;
    ctx.beginPath();
    ctx.arc(
      x + Math.cos(angle) * distance,
      y + Math.sin(angle * 1.13) * distance * 0.82,
      nucleusRadius * (0.035 + seededUnit(seed, i + 291) * 0.045),
      0,
      Math.PI * 2,
    );
    ctx.fill();
  }
}

function drawDropletOutline(radius, flags, stretchFix) {
  ctx.strokeStyle = flags.isMe
    ? "#ffffff"
    : flags.isSameOwner
      ? "rgba(255,255,255,0.72)"
      : flags.canEat
        ? "rgba(138,255,207,0.85)"
        : flags.canBeEaten
          ? "rgba(255,139,157,0.85)"
          : "rgba(255,255,255,0.34)";
  ctx.lineWidth = (flags.isMe ? 3 : flags.isSameOwner ? 2.4 : 1.5) * stretchFix;
  ctx.beginPath();
  ctx.arc(0, 0, radius, 0, Math.PI * 2);
  ctx.stroke();
}

function getDropletPalette(color, seed) {
  const rgb = parseHexColor(color) || { r: 138, g: 255, b: 207 };
  const accent = getCellAccentPalette(rgb, seed);
  const membraneLight = mixRgb(accent.membrane, { r: 255, g: 255, b: 255 }, 0.5);
  const membrane = mixRgb(accent.membrane, rgb, 0.22);
  const cytoplasmLight = mixRgb(accent.cytoplasm, { r: 255, g: 255, b: 255 }, 0.42);
  const cytoplasm = mixRgb(accent.cytoplasm, rgb, 0.2);
  const cytoplasmDeep = mixRgb(accent.cytoplasm, accent.membrane, 0.38);
  const nucleus = mixRgb(accent.nucleus, rgb, 0.12);
  const nucleusEdge = mixRgb(accent.nucleusEdge, rgb, 0.08);
  const granule = mixRgb(accent.granule, rgb, 0.16);
  return {
    cytoplasmLight: rgbToCss(cytoplasmLight, 0.62),
    cytoplasm: rgbToCss(cytoplasm, 0.52),
    cytoplasmDeep: rgbToCss(cytoplasmDeep, 0.48),
    membraneLight: rgbToCss(membraneLight, 0.56),
    membrane: rgbToCss(membrane, 0.42),
    membraneEdge: rgbToCss(mixRgb(accent.membrane, { r: 10, g: 22, b: 44 }, 0.42), 0.2),
    membraneStroke: rgbToCss(mixRgb(accent.membrane, { r: 255, g: 255, b: 255 }, 0.24), 0.5),
    rim: rgbToCss(mixRgb(accent.membrane, { r: 255, g: 255, b: 255 }, 0.48), 0.42),
    membraneBand: rgbToCss(mixRgb(accent.membrane, { r: 255, g: 255, b: 255 }, 0.28), 0.28),
    membranePore: rgbToCss(mixRgb(accent.membrane, { r: 255, g: 255, b: 255 }, 0.36), 0.4),
    strand: rgbToCss(mixRgb(accent.cytoplasm, { r: 255, g: 255, b: 255 }, 0.58), 0.46),
    vesicle: rgbToCss(mixRgb(accent.cytoplasm, { r: 255, g: 255, b: 255 }, 0.48), 0.24),
    vesicleRing: rgbToCss(mixRgb(accent.membrane, { r: 255, g: 255, b: 255 }, 0.4), 0.28),
    granule: rgbToCss(granule, 0.5),
    granuleLight: rgbToCss(mixRgb(accent.granule, { r: 255, g: 255, b: 255 }, 0.4), 0.4),
    granuleDark: rgbToCss(mixRgb(accent.nucleusEdge, { r: 18, g: 16, b: 24 }, 0.38), 0.58),
    nucleus: rgbToCss(nucleus, 0.92),
    nucleusLight: rgbToCss(mixRgb(accent.nucleus, { r: 255, g: 244, b: 240 }, 0.56), 0.9),
    nucleusEdge: rgbToCss(nucleusEdge, 0.88),
    nucleusRing: rgbToCss(mixRgb(accent.nucleus, { r: 255, g: 255, b: 255 }, 0.5), 0.48),
    nucleolus: "rgba(255, 241, 237, 0.46)",
    chromatin: rgbToCss(mixRgb(accent.nucleusEdge, accent.nucleus, 0.3), 0.34),
  };
}

function getCellAccentPalette(rgb, seed) {
  const baseHue = rgbToHue(rgb);
  const membraneHue = wrapHue(baseHue + (seededUnit(seed, 311) - 0.5) * 44);
  const cytoplasmHue = wrapHue(baseHue + 16 + (seededUnit(seed, 321) - 0.5) * 64);
  const nucleusHue = wrapHue(baseHue + 105 + seededUnit(seed, 331) * 138);
  const granuleHue = wrapHue(nucleusHue + 24 + (seededUnit(seed, 341) - 0.5) * 56);
  const membraneSaturation = 58 + seededUnit(seed, 351) * 24;
  const cytoplasmSaturation = 48 + seededUnit(seed, 361) * 26;
  const nucleusSaturation = 64 + seededUnit(seed, 371) * 22;
  return {
    membrane: hslToRgb(membraneHue, membraneSaturation, 64 + seededUnit(seed, 381) * 8),
    cytoplasm: hslToRgb(cytoplasmHue, cytoplasmSaturation, 68 + seededUnit(seed, 391) * 8),
    nucleus: hslToRgb(nucleusHue, nucleusSaturation, 57 + seededUnit(seed, 401) * 8),
    nucleusEdge: hslToRgb(nucleusHue, nucleusSaturation + 8, 28 + seededUnit(seed, 411) * 8),
    granule: hslToRgb(granuleHue, 48 + seededUnit(seed, 421) * 22, 42 + seededUnit(seed, 431) * 10),
  };
}

function parseHexColor(color) {
  if (typeof color !== "string" || !color.startsWith("#")) {
    return null;
  }
  const raw = color.length === 4
    ? color.slice(1).split("").map((part) => part + part).join("")
    : color.slice(1, 7);
  if (!/^[0-9a-f]{6}$/i.test(raw)) {
    return null;
  }
  return {
    r: parseInt(raw.slice(0, 2), 16),
    g: parseInt(raw.slice(2, 4), 16),
    b: parseInt(raw.slice(4, 6), 16),
  };
}

function mixRgb(a, b, amount) {
  return {
    r: Math.round(lerp(a.r, b.r, amount)),
    g: Math.round(lerp(a.g, b.g, amount)),
    b: Math.round(lerp(a.b, b.b, amount)),
  };
}

function rgbToCss(rgb, alpha = 1) {
  return `rgba(${rgb.r}, ${rgb.g}, ${rgb.b}, ${alpha})`;
}

function rgbToHue(rgb) {
  const r = rgb.r / 255;
  const g = rgb.g / 255;
  const b = rgb.b / 255;
  const max = Math.max(r, g, b);
  const min = Math.min(r, g, b);
  const delta = max - min;
  if (delta === 0) {
    return 0;
  }
  let hue;
  if (max === r) {
    hue = ((g - b) / delta) % 6;
  } else if (max === g) {
    hue = (b - r) / delta + 2;
  } else {
    hue = (r - g) / delta + 4;
  }
  return (hue * 60 + 360) % 360;
}

function wrapHue(hue) {
  return (hue % 360 + 360) % 360;
}

function hslToRgb(hue, saturation, lightness) {
  const h = wrapHue(hue) / 360;
  const s = clampRange(saturation, 0, 100) / 100;
  const l = clampRange(lightness, 0, 100) / 100;
  if (s === 0) {
    const value = Math.round(l * 255);
    return { r: value, g: value, b: value };
  }

  const hueToRgb = (p, q, t) => {
    let nextT = t;
    if (nextT < 0) nextT += 1;
    if (nextT > 1) nextT -= 1;
    if (nextT < 1 / 6) return p + (q - p) * 6 * nextT;
    if (nextT < 1 / 2) return q;
    if (nextT < 2 / 3) return p + (q - p) * (2 / 3 - nextT) * 6;
    return p;
  };

  const q = l < 0.5 ? l * (1 + s) : l + s - l * s;
  const p = 2 * l - q;
  return {
    r: Math.round(hueToRgb(p, q, h + 1 / 3) * 255),
    g: Math.round(hueToRgb(p, q, h) * 255),
    b: Math.round(hueToRgb(p, q, h - 1 / 3) * 255),
  };
}

function hashString(value) {
  let hash = 2166136261;
  for (let i = 0; i < value.length; i += 1) {
    hash ^= value.charCodeAt(i);
    hash = Math.imul(hash, 16777619);
  }
  return hash >>> 0;
}

function seededUnit(seed, salt) {
  let value = seed ^ Math.imul(salt + 1, 2246822519);
  value ^= value >>> 15;
  value = Math.imul(value, 3266489917);
  value ^= value >>> 16;
  return (value >>> 0) / 4294967295;
}

function drawBeneficialEffectRings(player, effects) {
  const baseRadius = player.drawRadius + 8;
  effects.forEach((effect, index) => {
    const ringRadius = baseRadius + index * 7;
    const pulse = 1 + Math.sin(state.time * 4.2 + player.drawX * 0.01 + index * 0.8) * 0.035;

    ctx.strokeStyle = effect.glow;
    ctx.lineWidth = 6;
    ctx.beginPath();
    ctx.arc(player.drawX, player.drawY, ringRadius * pulse, 0, Math.PI * 2);
    ctx.stroke();

    ctx.strokeStyle = effect.color;
    ctx.lineWidth = 2.6;
    ctx.beginPath();
    ctx.arc(player.drawX, player.drawY, ringRadius * pulse, 0, Math.PI * 2);
    ctx.stroke();
  });
}

function applySnapshotObjects(data) {
  const isFull = Boolean(data.full);
  if (isFull) {
    state.playerMap.clear();
    state.foodMap.clear();
    state.cactusMap.clear();
    state.leechMap.clear();
    state.wormholeMap.clear();
  }

  applyObjectDelta(state.playerMap, data.players, data.removedPlayerIds);
  applyObjectDelta(state.foodMap, data.foods, data.removedFoodIds);
  applyObjectDelta(state.cactusMap, data.cacti, data.removedCactusIds);
  applyObjectDelta(state.leechMap, data.leeches, data.removedLeechIds);
  applyObjectDelta(state.wormholeMap, data.wormholes, data.removedWormholeIds);

  state.players = Array.from(state.playerMap.values());
  state.foods = Array.from(state.foodMap.values());
  state.cacti = Array.from(state.cactusMap.values());
  state.leeches = Array.from(state.leechMap.values());
  state.wormholes = Array.from(state.wormholeMap.values());
}

function applyObjectDelta(targetMap, changedItems = [], removedIds = []) {
  for (const id of removedIds || []) {
    targetMap.delete(id);
  }
  for (const item of changedItems || []) {
    if (!item || !item.id) {
      continue;
    }
    targetMap.set(item.id, item);
  }
}

function handleBinarySnapshot(bufferLike) {
  const data = decodeSnapshotBinary(bufferLike);
  if (!data) {
    return;
  }

  const snapshotAt = performance.now();
  const previousMe = state.renderPlayers.get(state.playerId);
  if (state.lastSnapshotAt > 0) {
    state.snapshotGap = Math.max(16, Math.min(140, snapshotAt - state.lastSnapshotAt));
  }
  state.lastSnapshotAt = snapshotAt;
  applySnapshotObjects(data);
  syncRenderPlayers(state.players, snapshotAt);
  const me = state.renderPlayers.get(state.playerId);
  maybeShowBeneficialEffectMessage(previousMe, me);
  const grouped = state.leaderboard.length > 0 ? state.leaderboard : aggregateOwners(state.players);
  const myOwnerId = me ? (me.ownerId || me.id) : state.playerId;
  const myGroup = grouped.find((entry) => entry.ownerId === myOwnerId);
  if (me) {
    hudMass.textContent = isRespawningPlayer(me) ? `부활 ${Math.max(1, Math.ceil((me.respawnRemaining || 0) / 1000))}초` : Math.round(myGroup ? myGroup.mass : effectiveCombatMassClient(me));
    updateAbilityHud(me);
  }
  renderLeaderboard();
  renderChat();
}

function decodeSnapshotBinary(bufferLike) {
  const buffer = bufferLike instanceof ArrayBuffer ? bufferLike : bufferLike?.buffer;
  if (!buffer) {
    return null;
  }
  const view = new DataView(buffer);
  let offset = 0;

  if (
    view.byteLength < 25 ||
    view.getUint8(offset++) !== 83 ||
    view.getUint8(offset++) !== 78 ||
    view.getUint8(offset++) !== 80 ||
    view.getUint8(offset++) !== 50
  ) {
    return null;
  }

  const flags = view.getUint8(offset++);
  const full = (flags & 1) === 1;
  const counts = [];
  for (let i = 0; i < 10; i += 1) {
    counts.push(view.getUint16(offset, true));
    offset += 2;
  }

  const reader = {
    view,
    get offset() {
      return offset;
    },
    set offset(value) {
      offset = value;
    },
  };

  const players = [];
  for (let i = 0; i < counts[0]; i += 1) {
    players.push(readBinaryPlayer(reader));
  }
  const removedPlayerIds = [];
  for (let i = 0; i < counts[1]; i += 1) {
    removedPlayerIds.push(readBinaryString(reader));
  }
  const foods = [];
  for (let i = 0; i < counts[2]; i += 1) {
    foods.push(readBinaryFood(reader));
  }
  const removedFoodIds = [];
  for (let i = 0; i < counts[3]; i += 1) {
    removedFoodIds.push(readBinaryString(reader));
  }
  const cacti = [];
  for (let i = 0; i < counts[4]; i += 1) {
    cacti.push(readBinaryCactus(reader));
  }
  const removedCactusIds = [];
  for (let i = 0; i < counts[5]; i += 1) {
    removedCactusIds.push(readBinaryString(reader));
  }
  const leeches = [];
  for (let i = 0; i < counts[6]; i += 1) {
    leeches.push(readBinaryLeech(reader));
  }
  const removedLeechIds = [];
  for (let i = 0; i < counts[7]; i += 1) {
    removedLeechIds.push(readBinaryString(reader));
  }
  const wormholes = [];
  for (let i = 0; i < counts[8]; i += 1) {
    wormholes.push(readBinaryWormhole(reader));
  }
  const removedWormholeIds = [];
  for (let i = 0; i < counts[9]; i += 1) {
    removedWormholeIds.push(readBinaryString(reader));
  }

  return {
    type: "snapshot",
    full,
    players,
    removedPlayerIds,
    foods,
    removedFoodIds,
    cacti,
    removedCactusIds,
    leeches,
    removedLeechIds,
    wormholes,
    removedWormholeIds,
  };
}

function readBinaryPlayer(reader) {
  const id = readBinaryString(reader);
  const ownerId = readStringRef("ownerIds", readU16(reader));
  const nickname = readStringRef("nicknames", readU16(reader));
  const cellType = readStringRef("cellTypes", readU16(reader));
  const abilityName = readStringRef("abilityNames", readU16(reader));
  const color = readStringRef("colors", readU16(reader));
  const x = readQuantU16(reader, COORD_QUANT_SCALE);
  const y = readQuantU16(reader, COORD_QUANT_SCALE);
  const mass = readQuantU32(reader, MASS_QUANT_SCALE);
  const radius = readQuantU16(reader, RADIUS_QUANT_SCALE);
  const scale = readQuantU16(reader, SCALE_QUANT_SCALE);
  const isBot = readU8(reader) === 1;
  const coins = readU16(reader);
  const upgrades = decodeUpgradeBits(readU8(reader));
  const cooldownRemaining = readDurationU16(reader);
  const effectRemaining = readDurationU16(reader);
  const shieldRemaining = readDurationU16(reader);
  const probioticRemaining = readDurationU16(reader);
  const speedBoostRemaining = readDurationU16(reader);
  const respawnRemaining = readDurationU16(reader);
  return {
    id,
    ownerId,
    nickname,
    cellType,
    abilityName,
    color,
    x,
    y,
    mass,
    radius,
    scale,
    isBot,
    coins,
    upgrades,
    cooldownRemaining,
    effectRemaining,
    shieldRemaining,
    probioticRemaining,
    speedBoostRemaining,
    respawnRemaining,
  };
}

function applyStringTableUpdate(data) {
  applyStringEntries(state.strings.ownerIds, data.ownerIds);
  applyStringEntries(state.strings.nicknames, data.nicknames);
  applyStringEntries(state.strings.colors, data.colors);
  applyStringEntries(state.strings.abilityNames, data.abilityNames);
  applyStringEntries(state.strings.cellTypes, data.cellTypes);
}

function applyStringEntries(targetMap, entries = []) {
  for (const entry of entries || []) {
    if (!entry) {
      continue;
    }
    targetMap.set(Number(entry.id), entry.value || "");
  }
}

function readStringRef(kind, id) {
  return state.strings[kind].get(id) || "";
}

function readBinaryFood(reader) {
  return {
    id: readBinaryString(reader),
    x: readQuantU16(reader, COORD_QUANT_SCALE),
    y: readQuantU16(reader, COORD_QUANT_SCALE),
    radius: readQuantU16(reader, RADIUS_QUANT_SCALE),
    value: readQuantU16(reader, VALUE_QUANT_SCALE),
    kind: readBinaryString(reader),
  };
}

function readBinaryCactus(reader) {
  return {
    id: readBinaryString(reader),
    x: readQuantU16(reader, COORD_QUANT_SCALE),
    y: readQuantU16(reader, COORD_QUANT_SCALE),
    size: readQuantU16(reader, RADIUS_QUANT_SCALE),
    height: readQuantU16(reader, RADIUS_QUANT_SCALE),
  };
}

function readBinaryLeech(reader) {
  return {
    id: readBinaryString(reader),
    x: readQuantU16(reader, COORD_QUANT_SCALE),
    y: readQuantU16(reader, COORD_QUANT_SCALE),
    size: readQuantU16(reader, RADIUS_QUANT_SCALE),
    mass: readQuantU16(reader, MASS_QUANT_SCALE),
    angle: readQuantU16(reader, SCALE_QUANT_SCALE) - Math.PI * 2,
    attachedTo: readBinaryString(reader),
    attachedRemaining: readDurationU16(reader),
    burstRemaining: readDurationU16(reader),
  };
}

function readBinaryWormhole(reader) {
  return {
    id: readBinaryString(reader),
    kind: readBinaryString(reader),
    pairId: readBinaryString(reader),
    x: readQuantU16(reader, COORD_QUANT_SCALE),
    y: readQuantU16(reader, COORD_QUANT_SCALE),
    radius: readQuantU16(reader, RADIUS_QUANT_SCALE),
    pullRange: readQuantU16(reader, RADIUS_QUANT_SCALE),
  };
}

function readBinaryString(reader) {
  const length = readU16(reader);
  const bytes = new Uint8Array(reader.view.buffer, reader.offset, length);
  reader.offset += length;
  return snapshotTextDecoder.decode(bytes);
}

function readU8(reader) {
  const value = reader.view.getUint8(reader.offset);
  reader.offset += 1;
  return value;
}

function readU16(reader) {
  const value = reader.view.getUint16(reader.offset, true);
  reader.offset += 2;
  return value;
}

function readU32(reader) {
  const value = reader.view.getUint32(reader.offset, true);
  reader.offset += 4;
  return value;
}

function readQuantU16(reader, scale) {
  return readU16(reader) / scale;
}

function readQuantU32(reader, scale) {
  return readU32(reader) / scale;
}

function readDurationU16(reader) {
  return readU16(reader) * DURATION_QUANT_STEP_MS;
}

function decodeUpgradeBits(bits) {
  return {
    classic: (bits & (1 << 0)) !== 0,
    blink: (bits & (1 << 1)) !== 0,
    giant: (bits & (1 << 2)) !== 0,
    shield: (bits & (1 << 3)) !== 0,
    magnet: (bits & (1 << 4)) !== 0,
    divider: (bits & (1 << 5)) !== 0,
  };
}

function getFragmentDeformation(player) {
  const ownerId = player.ownerId || player.id;
  let nearest = null;
  let nearestDistance = Infinity;
  const velocityX = player.velocityX || 0;
  const velocityY = player.velocityY || 0;
  const speed = Math.hypot(velocityX, velocityY);
  const moveAngle = speed > 0.001 ? Math.atan2(velocityY, velocityX) : 0;
  const slimeWave = Math.sin(state.time * 7.2 + player.drawX * 0.01 + player.drawY * 0.013);
  const speedStretch = clamp01(speed / 260) * 0.08;
  const idlePulse = 0.012 + clamp01(speed / 320) * 0.016;

  for (const other of state.renderPlayers.values()) {
    if (other.id === player.id || (other.ownerId || other.id) !== ownerId) {
      continue;
    }
    const dist = Math.hypot(other.drawX - player.drawX, other.drawY - player.drawY);
    if (dist < nearestDistance) {
      nearestDistance = dist;
      nearest = other;
    }
  }

  if (!nearest) {
    const wobble = slimeWave * idlePulse;
    return {
      scaleX: 1 + speedStretch + wobble,
      scaleY: 1 - speedStretch * 0.68 - wobble * 0.55,
      angle: moveAngle,
    };
  }

  const combinedRadius = player.drawRadius + nearest.drawRadius;
  const squishStart = combinedRadius * 1.2;
  if (nearestDistance >= squishStart) {
    const wobble = slimeWave * idlePulse;
    return {
      scaleX: 1 + speedStretch + wobble,
      scaleY: 1 - speedStretch * 0.68 - wobble * 0.55,
      angle: moveAngle,
    };
  }

  const proximity = clamp01(1 - nearestDistance / squishStart);
  const amount = proximity * 0.14;
  const wobble = slimeWave * (idlePulse + proximity * 0.015);
  const baseAngle = Math.atan2(nearest.drawY - player.drawY, nearest.drawX - player.drawX);
  const angleBlend = proximity * 0.72;
  const angle = speed > 0.001
    ? lerpAngle(moveAngle, baseAngle, angleBlend)
    : baseAngle;

  return {
    scaleX: 1 + amount + speedStretch + wobble,
    scaleY: 1 - amount * 0.72 - speedStretch * 0.58 - wobble * 0.6,
    angle,
  };
}

function lerpAngle(start, end, alpha) {
  let diff = end - start;
  while (diff > Math.PI) diff -= Math.PI * 2;
  while (diff < -Math.PI) diff += Math.PI * 2;
  return start + diff * alpha;
}

function renderLeaderboard() {
  const grouped = (state.leaderboard.length > 0 ? [...state.leaderboard] : aggregateOwners(state.players).sort((a, b) => b.mass - a.mass));
  const myOwnerId = getMyOwnerId();
  const myIndex = grouped.findIndex((player) => player.ownerId === myOwnerId);
  hudRank.textContent = myIndex >= 0 ? `${myIndex + 1} / ${grouped.length}` : "-";
  const topEntries = grouped.slice(0, 6);
  const renderKey = `${myOwnerId}|${myIndex}|${topEntries.map((player) => `${player.ownerId}:${Math.round(player.mass)}`).join("|")}`;
  if (state.lastLeaderboardKey === renderKey) {
    return;
  }
  state.lastLeaderboardKey = renderKey;
  leaderboardContent.innerHTML = `
    <h2>리더보드</h2>
    ${topEntries.map((player, index) => `
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
  const renderKey = `${state.chatCollapsed ? "c" : "o"}|${items.map((entry) => entry.id).join("|")}`;
  if (state.lastChatRenderKey === renderKey) {
    return;
  }
  state.lastChatRenderKey = renderKey;
  if (items.length === 0) {
    chatMessages.innerHTML = `<div class="chat-entry"><div class="chat-text">아직 채팅이 없습니다.</div></div>`;
    return;
  }

  chatMessages.innerHTML = items.map((entry) => `
    <div class="chat-entry ${entry.isBot ? "bot" : ""}">
      <div class="chat-text"><span class="chat-name">${escapeHtml(entry.nickname)}</span>: ${escapeHtml(entry.message)}</div>
    </div>
  `).join("");
  chatMessages.scrollTop = chatMessages.scrollHeight;
}

function markChatActivity() {
  state.chatLastActivityAt = performance.now();
  chatPanel.classList.remove("faded");
}

function setChatCollapsed(collapsed) {
  state.chatCollapsed = collapsed;
  if (!collapsed) {
    markChatActivity();
  }
  chatPanel.classList.toggle("collapsed", collapsed);
  chatToggle.textContent = collapsed ? "채팅 열기" : "채팅 접기";
  chatToggle.setAttribute("aria-expanded", String(!collapsed));
}

function handleIncomingChats(nextChats) {
  const previousIds = new Set(state.chats.map((entry) => entry.id));
  const newEntries = nextChats.filter((entry) => !previousIds.has(entry.id));
  if (newEntries.length === 0) {
    return;
  }

  markChatActivity();
  if (state.chatCollapsed) {
    for (const entry of newEntries) {
      if (!isOwnChatEntry(entry)) {
        pushChatToast(entry);
      }
    }
  }
}

function isOwnChatEntry(entry) {
  return !!entry && entry.nickname === state.nickname && !entry.isBot;
}

function pushChatToast(entry) {
  if (!chatToast || !entry) {
    return;
  }
  const now = performance.now();
  state.chatToasts = state.chatToasts
    .filter((toast) => toast.id !== entry.id && toast.expiresAt > now)
    .concat({
      id: entry.id,
      text: `${entry.nickname}: ${entry.message}`,
      expiresAt: now + 5000,
    })
    .slice(-3);
  renderChatToasts();
}

function pruneChatToasts(now = performance.now()) {
  if (state.chatToasts.length === 0) {
    return false;
  }
  const nextToasts = state.chatToasts.filter((toast) => toast.expiresAt > now);
  if (nextToasts.length === state.chatToasts.length) {
    return false;
  }
  state.chatToasts = nextToasts;
  return true;
}

function renderChatToasts() {
  if (!chatToast) {
    return;
  }
  if (state.chatToasts.length === 0) {
    chatToast.innerHTML = "";
    chatToast.classList.add("hidden");
    return;
  }
  chatToast.classList.remove("hidden");
  chatToast.innerHTML = state.chatToasts
    .map((toast) => `<div class="chat-toast-line">${escapeHtml(toast.text)}</div>`)
    .join("");
}

function showMessage(text) {
  messageBox.textContent = text;
  messageBox.classList.remove("hidden");
  state.messageTimer = 2.2;
}

function maybeShowFullscreenPrompt() {
  if (!state.isTouchDevice || !fullscreenPrompt) {
    return;
  }
  if (isFullscreenActive() || isNativeAppShell()) {
    return;
  }
  fullscreenPrompt.classList.remove("hidden");
}

function hideFullscreenPrompt() {
  if (!fullscreenPrompt) {
    return;
  }
  fullscreenPrompt.classList.add("hidden");
}

function renderResetNotice() {
  if (!resetNotice) {
    return;
  }
  const resetAt = Number(state.resetAt);
  if (!Number.isFinite(resetAt) || resetAt <= 0) {
    resetNotice.classList.add("hidden");
    return;
  }

  const remainingMs = resetAt - Date.now();
  if (remainingMs > RESET_WARNING_WINDOW_MS || remainingMs <= 0) {
    resetNotice.classList.add("hidden");
    return;
  }

  const totalSeconds = Math.ceil(remainingMs / 1000);
  const minutes = Math.floor(totalSeconds / 60);
  const seconds = totalSeconds % 60;
  resetNotice.textContent = `초기화까지 ${minutes}:${String(seconds).padStart(2, "0")}`;
  resetNotice.classList.remove("hidden");
}

function renderUpgradePanel() {
  if (!upgradePanel) {
    return;
  }
  const me = getOwnedRepresentativePlayer();
  if (!me) {
    upgradePanel.classList.add("hidden");
    return;
  }

  const definition = UPGRADE_DEFS[me.cellType];
  if (!definition) {
    upgradePanel.classList.add("hidden");
    return;
  }

  const upgrades = me.upgrades || {};
  const owned = !!upgrades[me.cellType];
  const coins = Number(me.coins) || 0;
  const available = !owned && coins >= UPGRADE_COST;
  upgradeCoins.textContent = `${coins}코인`;
  upgradeName.textContent = definition.name;
  upgradeDescription.textContent = owned
    ? "구매 완료. 현재 세포 전용 업그레이드가 적용 중입니다."
    : `${definition.description} (${UPGRADE_COST}코인)`;
  upgradeBuy.disabled = !available;
  upgradeBuy.textContent = owned ? "구매 완료" : coins < UPGRADE_COST ? `${UPGRADE_COST}코인 필요` : "업그레이드 구매";
  upgradePanel.classList.toggle("available", available);
  upgradePanel.classList.remove("hidden");
  syncUpgradePanelState();
}

function syncUpgradePanelState() {
  if (!upgradeContent || !upgradeToggle) {
    return;
  }
  upgradeContent.classList.toggle("hidden", !state.upgradePanelOpen);
  upgradePanel.classList.toggle("open", state.upgradePanelOpen);
  upgradeToggle.setAttribute("aria-expanded", String(state.upgradePanelOpen));
  upgradeToggle.textContent = state.upgradePanelOpen ? "업그레이드 닫기" : "업그레이드 열기";
}

function getOwnedRepresentativePlayer() {
  const ownerId = getMyOwnerId();
  let best = null;
  for (const player of state.renderPlayers.values()) {
    if (isRespawningPlayer(player)) {
      continue;
    }
    if ((player.ownerId || player.id) !== ownerId) {
      continue;
    }
    if (!best || player.mass > best.mass) {
      best = player;
    }
  }
  return best;
}

function getOwnedFragmentCount() {
  const ownerId = getMyOwnerId();
  let count = 0;
  for (const player of state.renderPlayers.values()) {
    if (isRespawningPlayer(player)) {
      continue;
    }
    if ((player.ownerId || player.id) === ownerId) {
      count += 1;
    }
  }
  return count;
}

function renderControlHints() {
  const me = getOwnedRepresentativePlayer();
  if (!me) {
    if (touchMerge) {
      touchMerge.classList.add("hidden");
    }
    return;
  }

  const fragmentCount = getOwnedFragmentCount();
  const canManualMerge = me.cellType === "divider" && !!(me.upgrades || {}).divider && fragmentCount > 1;

  if (touchMerge) {
    touchMerge.classList.toggle("hidden", !canManualMerge);
  }

  if (!hudControls) {
    return;
  }

  const parts = ["Space 전용기", "W 배출"];
  if (canManualMerge) {
    parts.push("E 합체");
  }
  hudControls.textContent = `키 안내: ${parts.join(" / ")}`;
}

function isFullscreenActive() {
  if (isNativeAppShell()) {
    return true;
  }
  return !!(
    document.fullscreenElement ||
    document.webkitFullscreenElement ||
    document.msFullscreenElement
  );
}

function isNativeAppShell() {
  return window.__CELLGAME_NATIVE_APP__ === true || navigator.userAgent.includes("CellGameAndroidWebView");
}

async function requestMobileFullscreen() {
  if (isNativeAppShell()) {
    return true;
  }
  const target = document.documentElement;
  try {
    if (target.requestFullscreen) {
      await target.requestFullscreen({ navigationUI: "hide" });
      return true;
    }
    if (target.webkitRequestFullscreen) {
      target.webkitRequestFullscreen();
      return true;
    }
    if (target.msRequestFullscreen) {
      target.msRequestFullscreen();
      return true;
    }
  } catch {
    return false;
  }
  return false;
}

async function lockLandscapeOrientation() {
  if (isNativeAppShell()) {
    return true;
  }
  try {
    if (screen.orientation?.lock) {
      await screen.orientation.lock("landscape");
      return true;
    }
  } catch {
    return false;
  }
  return false;
}

function updateRotatePrompt() {
  if (!rotatePrompt || !state.isTouchDevice) {
    return;
  }
  if (isNativeAppShell()) {
    rotatePrompt.classList.add("hidden");
    return;
  }
  const orientationType = screen.orientation?.type || "";
  const isLandscapeByAPI = orientationType.startsWith("landscape");
  const viewportWidth = window.visualViewport?.width || window.innerWidth;
  const viewportHeight = window.visualViewport?.height || window.innerHeight;
  const isPortraitByViewport = viewportHeight > viewportWidth;
  const isPortrait = !isLandscapeByAPI && isPortraitByViewport;
  rotatePrompt.classList.toggle("hidden", !isPortrait);
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
  if (isRespawningPlayer(player)) {
    const remainingMs = player.respawnRemaining || 0;
    const remainingSeconds = Math.max(1, Math.ceil(remainingMs / 1000));
    hudCooldownFill.style.width = `${clamp01(1 - remainingMs / 5000) * 100}%`;
    hudCooldownLabel.textContent = "부활 대기";
    hudEffectFill.style.width = `${clamp01(remainingMs / 5000) * 100}%`;
    hudEffectLabel.textContent = `${remainingSeconds}초`;
    hudEffectFill.style.background = "";
    setBeneficialHudRows([]);
    return;
  }
  const cell = CELL_TYPES[player.cellType] || CELL_TYPES.classic;
  hudCellType.textContent = cell.name || player.cellType;
  hudAbilityName.textContent = player.abilityName || "없음";

  const cooldownRatio = cell.cooldownMs > 0 ? clamp01(1 - (player.cooldownRemaining / cell.cooldownMs)) : 1;
  hudCooldownFill.style.width = `${cooldownRatio * 100}%`;
  hudCooldownLabel.textContent = player.cooldownRemaining > 0 ? "재충전" : "준비";

  const probioticHud = getBeneficialHudState(player);
  if (probioticHud) {
    hudEffectFill.style.width = "0%";
    hudEffectFill.style.background = "";
    hudEffectLabel.textContent = "유익균";
    setBeneficialHudRows(probioticHud.effects);
    return;
  }

  const effectRatio = cell.effectMs > 0 ? clamp01(player.effectRemaining / cell.effectMs) : 0;
  hudEffectFill.style.width = `${effectRatio * 100}%`;
  hudEffectFill.style.background = "";
  hudEffectLabel.textContent = player.effectRemaining > 0 ? "활성" : "없음";
  setBeneficialHudRows([]);
}

function aggregateOwners(players) {
  const totals = new Map();
  for (const player of players) {
    if (isRespawningPlayer(player)) {
      continue;
    }
    const ownerId = player.ownerId || player.id;
    const existing = totals.get(ownerId);
    if (existing) {
        existing.mass += effectiveCombatMassClient(player);
        existing.fragments += 1;
        const combatMass = effectiveCombatMassClient(player);
        if (combatMass > existing.maxMass) {
          existing.maxMass = combatMass;
          existing.nickname = player.nickname;
        }
      } else {
      const combatMass = effectiveCombatMassClient(player);
        totals.set(ownerId, {
          ownerId,
          nickname: player.nickname,
          mass: combatMass,
          maxMass: combatMass,
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

function getOwnedMetricsFromRenderPlayers() {
  const ownerId = getMyOwnerId();
  let totalMass = 0;
  let x = 0;
  let y = 0;
  let maxRadius = 0;
  let fragmentCount = 0;
  let largestMass = 0;
  let focusX = 0;
  let focusY = 0;

  for (const player of state.renderPlayers.values()) {
    if (isRespawningPlayer(player)) {
      continue;
    }
    if ((player.ownerId || player.id) !== ownerId) {
      continue;
    }
    fragmentCount += 1;
    const mass = Math.max(1, player.mass);
    x += player.drawX * mass;
    y += player.drawY * mass;
    totalMass += mass;
    maxRadius = Math.max(maxRadius, player.drawRadius || player.radius || 0);
    if (mass >= largestMass) {
      largestMass = mass;
      focusX = player.drawX;
      focusY = player.drawY;
    }
  }

  if (totalMass <= 0) {
    return null;
  }

  return {
    x: fragmentCount > 1 ? focusX : x / totalMass,
    y: fragmentCount > 1 ? focusY : y / totalMass,
    totalMass,
    maxRadius,
  };
}

function toggleLeaderboardPanel() {
  state.leaderboardCollapsed = !state.leaderboardCollapsed;
  syncLeaderboardState();
}

function toggleMinimapPanel() {
  state.minimapCollapsed = !state.minimapCollapsed;
  syncMinimapState();
}

function toggleChatPanel() {
  setChatCollapsed(!state.chatCollapsed);
}

function bindTouchPanelToggle(button, toggle) {
  let lastTouchToggleAt = 0;
  button.addEventListener("pointerup", (event) => {
    if (!state.isTouchDevice || event.pointerType === "mouse") {
      return;
    }
    event.preventDefault();
    event.stopPropagation();
    lastTouchToggleAt = performance.now();
    toggle();
  });
  button.addEventListener("click", (event) => {
    if (state.isTouchDevice && performance.now() - lastTouchToggleAt < 450) {
      event.preventDefault();
      event.stopPropagation();
      return;
    }
    toggle();
  });
}

function syncLeaderboardState() {
  leaderboard.classList.toggle("collapsed", state.leaderboardCollapsed);
  leaderboardToggle.textContent = state.leaderboardCollapsed ? "순위 열기" : "순위 접기";
  leaderboardToggle.setAttribute("aria-expanded", String(!state.leaderboardCollapsed));
}

syncLeaderboardState();

function syncMinimapState() {
  minimap.classList.toggle("collapsed", state.minimapCollapsed);
  minimapToggle.textContent = state.minimapCollapsed ? "지도 열기" : "지도 접기";
  minimapToggle.setAttribute("aria-expanded", String(!state.minimapCollapsed));
}

syncMinimapState();

function getOwnedCenterFromPlayers() {
  const ownerId = getMyOwnerId();
  let totalMass = 0;
  let x = 0;
  let y = 0;

  for (const player of state.players) {
    if (isRespawningPlayer(player)) {
      continue;
    }
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

  const scaleX = width / world.width;
  const scaleY = height / world.height;
  const rangeX = canvas.width / Math.max(state.zoom, 0.01) * 0.5;
  const rangeY = canvas.height / Math.max(state.zoom, 0.01) * 0.5;

  minimapCtx.strokeStyle = "rgba(138,255,207,0.18)";
  const visionWidth = rangeX * 2 * scaleX;
  const visionHeight = rangeY * 2 * scaleY;
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
    if (Math.abs(dx) > rangeX || Math.abs(dy) > rangeY) {
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
    if (!isMine && (Math.abs(dx) > rangeX || Math.abs(dy) > rangeY)) {
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

function effectiveCombatMassClient(player) {
  return (player.mass || 0) * Math.max(1, player.scale || 1);
}

function hasShieldProtectionClient(player) {
  if ((player.shieldRemaining || 0) > 0) {
    return true;
  }
  return player.cellType === "shield" && (player.effectRemaining || 0) > 0;
}

function isRespawningPlayer(player) {
  return (player?.respawnRemaining || 0) > 0;
}

function formatRemainingSeconds(ms) {
  return `${Math.max(1, Math.ceil((ms || 0) / 1000))}초`;
}

function getBeneficialEffectEntries(player) {
  const effects = [];
  if ((player.probioticRemaining || 0) > 0) {
    const meta = BENEFICIAL_EFFECT_META.growth;
    effects.push({
      key: "growth",
      label: `${meta.label} ${formatRemainingSeconds(player.probioticRemaining)}`,
      remaining: player.probioticRemaining,
      duration: meta.duration,
      color: meta.color,
      fillColor: meta.fillColor,
      glow: meta.glow,
    });
  }
  if ((player.speedBoostRemaining || 0) > 0) {
    const meta = BENEFICIAL_EFFECT_META.speed;
    effects.push({
      key: "speed",
      label: `${meta.label} ${formatRemainingSeconds(player.speedBoostRemaining)}`,
      remaining: player.speedBoostRemaining,
      duration: meta.duration,
      color: meta.color,
      fillColor: meta.fillColor,
      glow: meta.glow,
    });
  }
  if ((player.shieldRemaining || 0) > 0) {
    const meta = BENEFICIAL_EFFECT_META.shield;
    effects.push({
      key: "shield",
      label: `${meta.label} ${formatRemainingSeconds(player.shieldRemaining)}`,
      remaining: player.shieldRemaining,
      duration: meta.duration,
      color: meta.color,
      fillColor: meta.fillColor,
      glow: meta.glow,
    });
  }
  return effects;
}

function getBeneficialHudState(player) {
  const effects = getBeneficialEffectEntries(player);
  if (effects.length === 0) {
    return null;
  }

  const primary = effects.reduce((best, current) => (current.remaining > best.remaining ? current : best));
  return {
    effects,
    ratio: clamp01(primary.remaining / primary.duration),
  };
}

function setBeneficialHudRows(effects) {
  const effectMap = new Map(effects.map((effect) => [effect.key, effect]));
  const rows = [
    ["growth", hudBuffGrowthRow, hudBuffGrowthFill],
    ["speed", hudBuffSpeedRow, hudBuffSpeedFill],
    ["shield", hudBuffShieldRow, hudBuffShieldFill],
  ];
  let hasAny = false;
  for (const [key, row, element] of rows) {
    const effect = effectMap.get(key);
    const ratio = effect ? clamp01(effect.remaining / effect.duration) : 0;
    element.style.width = `${ratio * 100}%`;
    row.classList.toggle("hidden", ratio <= 0);
    if (ratio > 0) {
      hasAny = true;
    }
  }
  hudBuffRows.classList.toggle("hidden", !hasAny);
}

function maybeShowBeneficialEffectMessage(previousPlayer, nextPlayer) {
  if (!nextPlayer || isRespawningPlayer(nextPlayer)) {
    return;
  }

  const gained = [];
  if ((nextPlayer.probioticRemaining || 0) > 0 && (previousPlayer?.probioticRemaining || 0) <= 0) {
    gained.push("성장");
  }
  if ((nextPlayer.speedBoostRemaining || 0) > 0 && (previousPlayer?.speedBoostRemaining || 0) <= 0) {
    gained.push("속도");
  }
  if ((nextPlayer.shieldRemaining || 0) > 0 && (previousPlayer?.shieldRemaining || 0) <= 0) {
    gained.push("보호");
  }

  if (gained.length > 0) {
    showMessage(`유익균 효과 획득: ${gained.join(", ")}`);
  }
}

function canPlayerEatClient(attacker, defender) {
  const gap = Math.hypot((attacker.drawX || attacker.x) - (defender.drawX || defender.x), (attacker.drawY || attacker.y) - (defender.drawY || defender.y));
  const attackerRadius = attacker.drawRadius || attacker.radius || 0;
  const defenderRadius = defender.drawRadius || defender.radius || 0;
  const requiredCenterDepth = attackerRadius - defenderRadius * 0.5;
  if (gap > requiredCenterDepth) {
    return false;
  }
  if (hasShieldProtectionClient(defender)) {
    return false;
  }
  if (effectiveCombatMassClient(attacker) <= effectiveCombatMassClient(defender) * 1.1) {
    return false;
  }
  if (defender.cellType === "giant" && (defender.effectRemaining || 0) > 0) {
    const requiredMass = (defender.mass || 0) * 1.1 * Math.max(1, defender.scale || 1);
    if (effectiveCombatMassClient(attacker) < requiredMass) {
      return false;
    }
  }
  if (attacker.cellType === "giant" && (attacker.effectRemaining || 0) > 0 && !(attacker.upgrades || {}).giant) {
    return false;
  }
  return true;
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
  const autoZoom = getAutoZoom();
  const desiredZoom = clampRange(autoZoom * state.zoomTarget, 0.1, 1.35);
  state.zoom = lerp(state.zoom, desiredZoom, 0.12);
}

function applyRuntimeConfig(config) {
  if (!config) {
    return;
  }
  const nextWorldSize = Number(config.worldSize);
  if (Number.isFinite(nextWorldSize) && nextWorldSize > 0) {
    world.width = nextWorldSize;
    world.height = nextWorldSize;
  }
}

function getViewportSafeSize() {
  const width = Math.max(320, canvas.width || window.visualViewport?.width || window.innerWidth || 0);
  const height = Math.max(240, canvas.height || window.visualViewport?.height || window.innerHeight || 0);
  const isLandscape = width >= height;

  if (state.isTouchDevice) {
    // Mobile controls are floating overlays, so we should not reserve large side gutters.
    const reservedWidth = isLandscape ? Math.min(width * 0.04, 42) : Math.min(width * 0.1, 56);
    const reservedHeight = isLandscape ? Math.min(height * 0.06, 26) : Math.min(height * 0.1, 48);
    return {
      width: Math.max(240, width - reservedWidth),
      height: Math.max(180, height - reservedHeight),
    };
  }

  const reservedWidth = Math.min(width * 0.18, 220);
  const reservedHeight = Math.min(height * 0.08, 70);
  return {
    width: Math.max(280, width - reservedWidth),
    height: Math.max(220, height - reservedHeight),
  };
}

function getAutoZoom() {
  const metrics = getOwnedMetricsFromRenderPlayers();
  if (!metrics) {
    return 1;
  }

  const dominantRadius = Math.max(24, metrics.maxRadius);
  const massRoot = Math.sqrt(metrics.totalMass);
  const safeViewport = getViewportSafeSize();
  const aspect = safeViewport.width / Math.max(1, safeViewport.height);
  const isMobileLandscape = state.isTouchDevice && safeViewport.width >= safeViewport.height;
  const horizontalSpan = Math.max(
    dominantRadius * 3.4,
    280 + dominantRadius * 5.1 + massRoot * 6.4,
  );
  const verticalSpan = Math.max(
    dominantRadius * 3.1,
    250 + dominantRadius * 4.6 + massRoot * 5.6,
  );
  const aspectCompensation = clampRange(aspect, 0.72, isMobileLandscape ? 1.9 : 1.45);
  const horizontalBias = isMobileLandscape ? 0.9 : 0.92;
  const verticalBias = isMobileLandscape ? 1.0 : 1.08;
  const zoomX = safeViewport.width / (horizontalSpan * Math.max(1, aspectCompensation * horizontalBias));
  const zoomY = safeViewport.height / (verticalSpan * Math.max(1, verticalBias / aspectCompensation));
  const minZoom = isMobileLandscape ? 0.22 : 0.16;
  const maxZoom = isMobileLandscape ? 1.12 : 1.05;
  return clampRange(Math.min(zoomX, zoomY), minZoom, maxZoom);
}

resizeCanvas();
