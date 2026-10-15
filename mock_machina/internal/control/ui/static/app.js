(function () {
  var root = document.documentElement;
  var themeKey = "mockmachina-theme";
  try {
    var saved = localStorage.getItem(themeKey);
    if (saved === "light" || saved === "dark") root.dataset.theme = saved;
  } catch (e) {}

  var connection = "connecting";
  var paused = false;
  var missed = 0;
  var pendingG = 0;
  var goKeys = { o: "", r: "routes", t: "traffic", d: "data", s: "scenarios", c: "coverage" };

  function byId(id) { return document.getElementById(id); }
  function base() { return document.body.dataset.base || "/__mockmachina/ui/"; }
  function typing(el) { return el && el.closest && el.closest("input, textarea, select, [contenteditable]"); }

  function showConnection() {
    var el = byId("connection");
    if (!el) return;
    el.dataset.state = connection;
    el.textContent = connection === "live" ? "live" : connection === "disconnected" ? "disconnected, retrying" : "connecting";
  }

  function showPause() {
    var button = byId("traffic-pause");
    if (!button) return;
    button.setAttribute("aria-pressed", paused ? "true" : "false");
    button.textContent = paused ? (missed ? "Resume (" + missed + " new)" : "Resume") : "Pause";
  }

  function togglePause() {
    if (!byId("traffic")) return;
    paused = !paused;
    if (!paused && missed && window.htmx) {
      htmx.ajax("GET", location.pathname + location.search, { target: "#traffic", select: "#traffic", swap: "outerHTML" });
    }
    missed = 0;
    showPause();
  }

  function openDialog(id) {
    var dialog = byId(id);
    if (!dialog || dialog.open) return;
    document.querySelectorAll("dialog[open]").forEach(function (d) { d.close(); });
    dialog.showModal();
    if (id === "palette") {
      var input = byId("palette-input");
      input.value = "";
      input.focus();
    }
  }

  function paletteLinks() { return Array.prototype.slice.call(document.querySelectorAll("#palette-results a")); }

  function selectLink(links, index) {
    links.forEach(function (a, i) { a.setAttribute("aria-selected", i === index ? "true" : "false"); });
    if (links[index]) links[index].scrollIntoView({ block: "nearest" });
  }

  function paletteKeys(event) {
    var links = paletteLinks();
    var current = links.findIndex(function (a) { return a.getAttribute("aria-selected") === "true"; });
    if (event.key === "ArrowDown" || event.key === "ArrowUp") {
      event.preventDefault();
      var step = event.key === "ArrowDown" ? 1 : -1;
      selectLink(links, Math.min(Math.max(current + step, 0), links.length - 1));
    } else if (event.key === "Enter" && links.length) {
      event.preventDefault();
      location.href = links[Math.max(current, 0)].href;
    }
  }

  function addCopyButtons(scope) {
    scope.querySelectorAll("pre[data-copy]").forEach(function (pre) {
      if (pre.querySelector(".copy")) return;
      var button = document.createElement("button");
      button.type = "button";
      button.className = "copy";
      button.textContent = "Copy";
      pre.appendChild(button);
    });
  }

  function copy(button) {
    var code = button.parentElement.querySelector("code");
    var done = function () { button.textContent = "Copied"; setTimeout(function () { button.textContent = "Copy"; }, 1500); };
    var select = function () {
      var range = document.createRange();
      range.selectNodeContents(code);
      var selection = getSelection();
      selection.removeAllRanges();
      selection.addRange(range);
      button.textContent = "Selected, press ⌘C";
    };
    if (navigator.clipboard && navigator.clipboard.writeText) {
      navigator.clipboard.writeText(code.textContent).then(done, select);
    } else {
      select();
    }
  }

  document.addEventListener("htmx:sseOpen", function () { connection = "live"; showConnection(); });
  document.addEventListener("htmx:sseError", function () { connection = "disconnected"; showConnection(); });
  document.addEventListener("htmx:afterSwap", function (event) {
    showConnection();
    showPause();
    if (event.target.id === "palette-results") selectLink(paletteLinks(), 0);
  });
  document.addEventListener("htmx:load", function (event) { addCopyButtons(event.detail.elt); });

  document.addEventListener("htmx:sseBeforeMessage", function (event) {
    if (paused && event.target.id === "traffic") {
      event.preventDefault();
      missed++;
      showPause();
    }
  });
  document.addEventListener("htmx:sseMessage", function (event) {
    var list = event.target;
    var cap = Number(list.dataset && list.dataset.cap);
    while (cap > 0 && list.children.length > cap) list.lastElementChild.remove();
  });

  document.addEventListener("click", function (event) {
    var target = event.target;
    if (target.closest("#theme-toggle")) {
      var dark = root.dataset.theme ? root.dataset.theme === "dark" : matchMedia("(prefers-color-scheme: dark)").matches;
      root.dataset.theme = dark ? "light" : "dark";
      try { localStorage.setItem(themeKey, root.dataset.theme); } catch (e) {}
    } else if (target.closest("#palette-open")) {
      openDialog("palette");
    } else if (target.closest("#traffic-pause")) {
      togglePause();
    } else if (target.closest("#traffic-clear")) {
      var list = byId("traffic");
      if (list) list.replaceChildren();
    } else if (target.closest(".copy")) {
      copy(target.closest(".copy"));
    } else if (target.matches("dialog")) {
      target.close();
    }
  });

  document.addEventListener("keydown", function (event) {
    if (event.target.id === "palette-input") {
      paletteKeys(event);
      return;
    }
    if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "k") {
      event.preventDefault();
      openDialog("palette");
      return;
    }
    if (event.metaKey || event.ctrlKey || event.altKey || typing(event.target) || document.querySelector("dialog[open]")) return;
    if (pendingG && Object.prototype.hasOwnProperty.call(goKeys, event.key)) {
      pendingG = 0;
      location.href = base() + goKeys[event.key];
      return;
    }
    pendingG = event.key === "g" ? Date.now() + 1000 : 0;
    if (event.key === "/") {
      event.preventDefault();
      var filter = document.querySelector("#route-filter input[type=search]");
      if (filter) filter.focus(); else openDialog("palette");
    } else if (event.key === "?") {
      openDialog("shortcuts");
    } else if (event.key === "p") {
      togglePause();
    }
  });
  setInterval(function () { if (pendingG && Date.now() > pendingG) pendingG = 0; }, 500);
})();
