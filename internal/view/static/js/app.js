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

  function ask(question, okLabel, onOk, onCancel) {
    var dialog = document.getElementById("confirm");

    document.getElementById("confirm-message").textContent = question;
    document.getElementById("confirm-ok").textContent = okLabel || "Да";
    dialog.returnValue = "";

    dialog.addEventListener(
      "close",
      function () {
        if (dialog.returnValue === "ok") {
          onOk();
        } else if (onCancel) {
          onCancel();
        }
      },
      { once: true }
    );

    dialog.showModal();
  }

  document.addEventListener("htmx:confirm", function (event) {
    if (!event.detail.question || !document.getElementById("confirm")) {
      return;
    }

    event.preventDefault();

    ask(event.detail.question, event.detail.elt.dataset.confirmOk, function () {
      event.detail.issueRequest(true);
    });
  });

  document.addEventListener("htmx:confirm", function (event) {
    var form = event.detail.elt;

    if (!form.matches("form[data-confirm-empty]") || !document.getElementById("confirm")) {
      return;
    }

    var input = form.querySelector("input[name=date]");

    if (input.value.trim() !== "") {
      return;
    }

    event.preventDefault();

    ask(
      form.dataset.confirmEmpty,
      "Удалить",
      function () {
        event.detail.issueRequest(true);
      },
      function () {
        input.value = input.defaultValue;
      }
    );
  });
})();

(function () {
  "use strict";

  function cancelEdit(form) {
    if (!form.dataset.cancel) {
      return;
    }

    htmx.ajax("GET", form.dataset.cancel, {
      source: form,
      target: form.getAttribute("hx-target"),
      swap: "outerHTML",
    });
  }

  document.addEventListener("click", function (event) {
    var form = document.querySelector("form[data-edit-form]");

    if (!form) {
      return;
    }

    var row = form.closest("li") || form;

    if (row.contains(event.target)) {
      return;
    }

    if (event.target.closest("a, button, input, select, textarea, label, summary, dialog")) {
      return;
    }

    cancelEdit(form);
  });

  document.addEventListener("keydown", function (event) {
    if (event.key !== "Escape") {
      return;
    }

    var form = event.target.closest("form[data-edit-form]");

    if (form) {
      cancelEdit(form);
    }
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

(function () {
  "use strict";

  var saved = null;

  document.addEventListener("htmx:beforeRequest", function () {
    var scroller = document.querySelector(".journal-scroll");
    saved = scroller ? { top: scroller.scrollTop, left: scroller.scrollLeft } : null;
  });

  function measure(table) {
    var scroller = table.parentElement;
    var head = table.tHead;
    var corner = head && head.rows[0] && head.rows[0].cells[0];

    if (!corner || scroller.dataset.measured) {
      return;
    }

    scroller.dataset.measured = "true";

    if (saved) {
      scroller.scrollTop = saved.top;
      scroller.scrollLeft = saved.left;
    }

    function update() {
      scroller.style.setProperty("--journal-head", head.offsetHeight + "px");
      scroller.style.setProperty("--journal-side", corner.offsetWidth + "px");
    }

    update();

    var highlighted = table.querySelector("tr[data-highlight]");
    if (highlighted) {
      highlighted.scrollIntoView({ block: "nearest" });
    }

    if (window.ResizeObserver) {
      var observer = new ResizeObserver(update);
      observer.observe(head);
      observer.observe(corner);
    }
  }

  function measureAll(root) {
    root.querySelectorAll(".journal-table").forEach(measure);
  }

  measureAll(document);

  document.addEventListener("htmx:load", function (event) {
    measureAll(event.detail.elt);
  });
})();

(function () {
  "use strict";

  function modal() {
    return document.getElementById("journal-modal");
  }

  document.addEventListener("htmx:afterSwap", function (event) {
    var dialog = modal();

    if (!dialog || event.detail.target.id !== "modal-body") {
      return;
    }

    if (event.detail.target.children.length === 0) {
      if (dialog.open) {
        dialog.close();
      }

      return;
    }

    if (!dialog.open) {
      dialog.showModal();
    }

    var focus = event.detail.target.querySelector("[autofocus]");
    if (focus) {
      focus.focus();
      if (focus.tagName === "INPUT") {
        focus.select();
      }
    }
  });

  document.addEventListener("click", function (event) {
    var dialog = modal();

    if (dialog && (event.target === dialog || event.target.closest("[data-modal-close]"))) {
      dialog.close();
    }
  });

  document.addEventListener("journal:close-modal", function () {
    var dialog = modal();

    if (dialog && dialog.open) {
      dialog.close();
    }
  });

  var dialog = modal();
  if (dialog) {
    dialog.addEventListener("close", function () {
      document.getElementById("modal-body").replaceChildren();
    });
  }
})();

(function () {
  "use strict";

  function pad(value) {
    return value.length === 1 ? "0" + value : value;
  }

  function isoDate(text) {
    var parts = /^(\d{1,2})\.(\d{1,2})\.(\d{4})$/.exec(text.trim());

    return parts ? parts[3] + "-" + pad(parts[2]) + "-" + pad(parts[1]) : "";
  }

  function textInput(wrapper) {
    return wrapper.querySelector("input:not([data-date-native])");
  }

  document.addEventListener("click", function (event) {
    var button = event.target.closest("[data-date-pick]");

    if (!button) {
      return;
    }

    var wrapper = button.closest("[data-date]");
    var native = wrapper.querySelector("[data-date-native]");
    var input = textInput(wrapper);

    native.value = isoDate(input.value);

    try {
      native.showPicker();
    } catch (error) {
      input.focus();
    }
  });

  function stopNative(event) {
    if (event.target.matches && event.target.matches("[data-date-native]")) {
      event.stopPropagation();
      return true;
    }

    return false;
  }

  document.addEventListener("input", stopNative, true);

  document.addEventListener(
    "change",
    function (event) {
      if (!stopNative(event)) {
        return;
      }

      var parts = event.target.value.split("-");
      var input = textInput(event.target.closest("[data-date]"));

      if (parts.length !== 3) {
        return;
      }

      input.value = parts[2] + "." + parts[1] + "." + parts[0];
      input.dispatchEvent(new Event("change", { bubbles: true }));
    },
    true
  );
})();

(function () {
  "use strict";

  document.addEventListener("change", function (event) {
    var input = event.target;

    if (!input.matches || !input.matches("input[data-file-names]")) {
      return;
    }

    var list = input.closest("form").querySelector("[data-file-list]");

    if (list) {
      list.textContent = Array.prototype.map
        .call(input.files, function (file) {
          return file.name;
        })
        .join(", ");
    }
  });
})();
