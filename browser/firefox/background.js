const extensionAPI = typeof browser !== "undefined" ? browser : chrome;

const STORAGE_KEY_BASE_URL = "serverBaseURL";
const STORAGE_KEY_DEVICE_ID = "deviceID";
const DEFAULT_BASE_URL = "http://127.0.0.1:8080";
const INGEST_PATH = "/api/ingest/browser-pages";

const lastUploadKeyByTab = new Map();

async function ensureSettings() {
  const current = await extensionAPI.storage.local.get([
    STORAGE_KEY_BASE_URL,
    STORAGE_KEY_DEVICE_ID
  ]);

  const updates = {};
  if (!current[STORAGE_KEY_BASE_URL]) {
    updates[STORAGE_KEY_BASE_URL] = DEFAULT_BASE_URL;
  }
  if (!current[STORAGE_KEY_DEVICE_ID]) {
    updates[STORAGE_KEY_DEVICE_ID] = generateDeviceID();
  }
  if (Object.keys(updates).length > 0) {
    await extensionAPI.storage.local.set(updates);
  }

  return {
    serverBaseURL: normalizeBaseURL(current[STORAGE_KEY_BASE_URL] || updates[STORAGE_KEY_BASE_URL] || DEFAULT_BASE_URL),
    deviceID: current[STORAGE_KEY_DEVICE_ID] || updates[STORAGE_KEY_DEVICE_ID]
  };
}

function generateDeviceID() {
  const bytes = new Uint8Array(8);
  crypto.getRandomValues(bytes);
  return Array.from(bytes, function(byte) {
    return byte.toString(16).padStart(2, "0");
  }).join("");
}

function normalizeBaseURL(rawURL) {
  const value = String(rawURL || "").trim();
  if (!value) {
    return DEFAULT_BASE_URL;
  }
  return value.replace(/\/+$/, "");
}

function shouldIgnoreURL(rawURL) {
  if (!rawURL) {
    return true;
  }
  if (!(rawURL.startsWith("http://") || rawURL.startsWith("https://"))) {
    return true;
  }

  try {
    const parsedURL = new URL(rawURL);
    return isLoopbackHost(parsedURL.hostname);
  } catch (_) {
    return true;
  }
}

function isLoopbackHost(hostname) {
  const normalizedHost = String(hostname || "").trim().toLowerCase();
  return normalizedHost === "localhost" ||
    normalizedHost === "127.0.0.1" ||
    normalizedHost === "::1" ||
    normalizedHost === "[::1]";
}

function buildUploadKey(payload) {
  return payload.url + ":" + payload.domHash;
}

async function uploadCapture(tabId, payload) {
  if (!payload || shouldIgnoreURL(payload.url) || !payload.html) {
    return;
  }

  const uploadKey = buildUploadKey(payload);
  if (lastUploadKeyByTab.get(tabId) === uploadKey) {
    return;
  }

  const settings = await ensureSettings();
  const response = await fetch(settings.serverBaseURL + INGEST_PATH, {
    method: "POST",
    headers: {
      "Content-Type": "application/json"
    },
    body: JSON.stringify({
      url: payload.url,
      final_url: payload.url,
      title: payload.title || "",
      captured_at: payload.capturedAt,
      html: payload.html,
      browser: "firefox",
      device_id: settings.deviceID
    })
  });

  if (!response.ok) {
    console.error("LLocalSearch upload failed", response.status, await response.text());
    return;
  }

  lastUploadKeyByTab.set(tabId, uploadKey);
}

async function requestCapture(tabId, reason) {
  if (typeof tabId !== "number" || tabId < 0) {
    return;
  }

  try {
    await extensionAPI.tabs.sendMessage(tabId, {
      type: "REQUEST_CAPTURE",
      reason: reason
    });
  } catch (error) {
    if (!String(error && error.message || "").includes("Receiving end does not exist")) {
      console.debug("LLocalSearch capture request skipped", error);
    }
  }
}

extensionAPI.runtime.onInstalled.addListener(function() {
  void ensureSettings();
});

extensionAPI.runtime.onStartup.addListener(function() {
  void ensureSettings();
});

extensionAPI.runtime.onMessage.addListener(function(message, sender) {
  if (!message || message.type !== "PAGE_CAPTURE") {
    return undefined;
  }

  const tabId = sender && sender.tab ? sender.tab.id : -1;
  return uploadCapture(tabId, message.payload);
});

extensionAPI.tabs.onUpdated.addListener(function(tabId, changeInfo, tab) {
  if (changeInfo.status !== "complete" || !tab || shouldIgnoreURL(tab.url)) {
    return;
  }
  void requestCapture(tabId, "tab-complete");
});

extensionAPI.webNavigation.onHistoryStateUpdated.addListener(function(details) {
  if (!details || details.frameId !== 0 || shouldIgnoreURL(details.url)) {
    return;
  }
  void requestCapture(details.tabId, "history-state-updated");
});

extensionAPI.tabs.onRemoved.addListener(function(tabId) {
  lastUploadKeyByTab.delete(tabId);
});
