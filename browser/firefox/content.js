(function() {
  const extensionAPI = typeof browser !== "undefined" ? browser : chrome;
  const CAPTURE_DEBOUNCE_MS = 1200;
  const URL_POLL_MS = 1000;

  let pendingTimer = null;
  let lastObservedURL = location.href;

  function shouldIgnoreURL(rawURL) {
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

  function hashString(value) {
    let hash = 2166136261;
    for (let index = 0; index < value.length; index += 1) {
      hash ^= value.charCodeAt(index);
      hash = Math.imul(hash, 16777619);
    }
    return (hash >>> 0).toString(16);
  }

  function buildPayload(reason) {
    if (!document.documentElement) {
      return null;
    }

    const url = location.href;
    if (shouldIgnoreURL(url)) {
      return null;
    }

    const html = document.documentElement.outerHTML;
    if (!html) {
      return null;
    }

    return {
      url: url,
      title: document.title || "",
      html: html,
      domHash: hashString(url + "\n" + html),
      capturedAt: new Date().toISOString(),
      reason: reason
    };
  }

  function sendCapture(reason) {
    const payload = buildPayload(reason);
    if (!payload) {
      return;
    }

    extensionAPI.runtime.sendMessage({
      type: "PAGE_CAPTURE",
      payload: payload
    }).catch(function(error) {
      console.debug("LLocalSearch could not send page capture", error);
    });
  }

  function scheduleCapture(reason, delay) {
    clearTimeout(pendingTimer);
    pendingTimer = setTimeout(function() {
      sendCapture(reason);
    }, delay);
  }

  const observer = new MutationObserver(function() {
    scheduleCapture("dom-mutated", CAPTURE_DEBOUNCE_MS);
  });

  if (document.documentElement) {
    observer.observe(document.documentElement, {
      childList: true,
      subtree: true,
      characterData: true
    });
  }

  window.addEventListener("load", function() {
    scheduleCapture("window-load", 800);
  });

  document.addEventListener("readystatechange", function() {
    if (document.readyState === "complete") {
      scheduleCapture("ready-state-complete", 800);
    }
  });

  setInterval(function() {
    if (location.href === lastObservedURL) {
      return;
    }
    lastObservedURL = location.href;
    scheduleCapture("url-changed", 500);
  }, URL_POLL_MS);

  extensionAPI.runtime.onMessage.addListener(function(message) {
    if (!message || message.type !== "REQUEST_CAPTURE") {
      return undefined;
    }
    scheduleCapture(message.reason || "background-request", 300);
    return undefined;
  });

  scheduleCapture("document-idle", 1000);
})();
