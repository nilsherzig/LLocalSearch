const extensionAPI = typeof browser !== "undefined" ? browser : chrome;

const STORAGE_KEY_BASE_URL = "serverBaseURL";
const STORAGE_KEY_DEVICE_ID = "deviceID";
const DEFAULT_BASE_URL = "http://127.0.0.1:8080";

function normalizeBaseURL(rawURL) {
  const value = String(rawURL || "").trim();
  if (!value) {
    return DEFAULT_BASE_URL;
  }
  return value.replace(/\/+$/, "");
}

function generateDeviceID() {
  const bytes = new Uint8Array(8);
  crypto.getRandomValues(bytes);
  return Array.from(bytes, function(byte) {
    return byte.toString(16).padStart(2, "0");
  }).join("");
}

async function loadSettings() {
  const stored = await extensionAPI.storage.local.get([
    STORAGE_KEY_BASE_URL,
    STORAGE_KEY_DEVICE_ID
  ]);

  const serverBaseURL = normalizeBaseURL(stored[STORAGE_KEY_BASE_URL] || DEFAULT_BASE_URL);
  const deviceID = stored[STORAGE_KEY_DEVICE_ID] || generateDeviceID();
  await extensionAPI.storage.local.set({
    [STORAGE_KEY_BASE_URL]: serverBaseURL,
    [STORAGE_KEY_DEVICE_ID]: deviceID
  });
  return {
    serverBaseURL: serverBaseURL,
    deviceID: deviceID
  };
}

function updateEndpointPreview(serverBaseURL) {
  document.getElementById("endpoint-preview").textContent =
    "Browser ingest endpoint: " + normalizeBaseURL(serverBaseURL) + "/api/ingest/browser-pages";
}

async function initializeOptions() {
  const settings = await loadSettings();
  document.getElementById("server-base-url").value = settings.serverBaseURL;
  document.getElementById("device-id").value = settings.deviceID;
  updateEndpointPreview(settings.serverBaseURL);
}

document.getElementById("server-base-url").addEventListener("input", function(event) {
  updateEndpointPreview(event.target.value);
});

document.getElementById("settings-form").addEventListener("submit", async function(event) {
  event.preventDefault();
  const serverBaseURL = normalizeBaseURL(document.getElementById("server-base-url").value);
  const deviceID = document.getElementById("device-id").value || generateDeviceID();

  await extensionAPI.storage.local.set({
    [STORAGE_KEY_BASE_URL]: serverBaseURL,
    [STORAGE_KEY_DEVICE_ID]: deviceID
  });

  document.getElementById("server-base-url").value = serverBaseURL;
  document.getElementById("device-id").value = deviceID;
  updateEndpointPreview(serverBaseURL);
  document.getElementById("status").textContent = "Saved.";
});

void initializeOptions();
