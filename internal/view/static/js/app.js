(function () {
  "use strict";

  document.addEventListener("click", function (event) {
    var button = event.target.closest("button[data-toggle-password]");

    if (!button) {
      return;
    }

    var input = button.parentElement.querySelector("input");
    var show = input.type === "password";

    input.type = show ? "text" : "password";
    button.setAttribute("aria-pressed", String(show));
    button.setAttribute("aria-label", show ? "Скрыть пароль" : "Показать пароль");
    button.querySelector("[data-eye]").hidden = show;
    button.querySelector("[data-eye-off]").hidden = !show;
    input.focus();
  });
})();

(function () {
  "use strict";

  document.addEventListener("htmx:confirm", function (event) {
    var dialog = document.getElementById("confirm");

    if (!event.detail.question || !dialog) {
      return;
    }

    event.preventDefault();

    document.getElementById("confirm-message").textContent = event.detail.question;
    document.getElementById("confirm-ok").textContent = event.detail.elt.dataset.confirmOk || "Да";
    dialog.returnValue = "";

    dialog.addEventListener(
      "close",
      function () {
        if (dialog.returnValue === "ok") {
          event.detail.issueRequest(true);
        }
      },
      { once: true }
    );

    dialog.showModal();
  });
})();

(function () {
  "use strict";

  var toggle = document.getElementById("nav-toggle");
  var sidebar = document.getElementById("sidebar");
  var overlay = document.getElementById("nav-overlay");

  if (!toggle || !sidebar || !overlay) {
    return;
  }

  function setOpen(open) {
    var state = open ? "open" : "closed";

    sidebar.dataset.state = state;
    overlay.dataset.state = state;
    toggle.setAttribute("aria-expanded", String(open));
    document.documentElement.toggleAttribute("data-nav-open", open);

    if (open) {
      sidebar.removeAttribute("inert");
    } else if (window.matchMedia("(max-width: 1023px)").matches) {
      sidebar.setAttribute("inert", "");
    }
  }

  function isOpen() {
    return sidebar.dataset.state === "open";
  }

  setOpen(false);

  toggle.addEventListener("click", function () {
    setOpen(!isOpen());
  });

  overlay.addEventListener("click", function () {
    setOpen(false);
  });

  sidebar.addEventListener("click", function (event) {
    if (event.target.closest("a")) {
      setOpen(false);
    }
  });

  document.addEventListener("keydown", function (event) {
    if (event.key === "Escape" && isOpen()) {
      setOpen(false);
      toggle.focus();
    }
  });

  window.matchMedia("(min-width: 1024px)").addEventListener("change", function (event) {
    if (event.matches) {
      setOpen(false);
      sidebar.removeAttribute("inert");
    } else {
      setOpen(false);
    }
  });
})();
