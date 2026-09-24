(function () {
  "use strict";

  var input = document.getElementById("book-search");
  var grid = document.getElementById("book-grid");
  var countEl = document.getElementById("result-count");
  var noResults = document.getElementById("no-results");

  if (input && grid) {
    var cards = Array.prototype.slice.call(grid.querySelectorAll(".book-card"));
    var total = cards.length;

    function normalize(text) {
      return (text || "")
        .toLowerCase()
        .normalize("NFD")
        .replace(/[\u0300-\u036f]/g, "");
    }

    function filter() {
      var query = normalize(input.value.trim());
      var visible = 0;

      cards.forEach(function (card) {
        var haystack = normalize(card.dataset.title + " " + card.dataset.author);
        var matches = query === "" || haystack.indexOf(query) !== -1;
        card.classList.toggle("d-none", !matches);
        if (matches) visible++;
      });

      if (countEl) {
        countEl.textContent = "Mostrando " + visible + " de " + total + " libros";
      }
      if (noResults) {
        noResults.classList.toggle("d-none", visible !== 0);
      }
    }

    input.addEventListener("input", filter);
  }

  var flash = document.getElementById("flash");
  var flashText = document.getElementById("flash-text");

  function showFlash(style, text) {
    if (!flash || !flashText) return;
    flash.className = "alert alert-dismissible fade show " + style;
    flashText.textContent = text;
  }

  var flashMessages = {
    "ok-agregado": ["alert-success", "Libro agregado al catálogo."],
    "ok-editado": ["alert-success", "Libro actualizado correctamente."],
    "ok-eliminado": ["alert-success", "Libro eliminado del catálogo."]
  };

  (function initFlash() {
    if (!flash) return;
    var params = new URLSearchParams(window.location.search);
    var msg = params.get("msg");
    if (!msg) return;
    var motivo = params.get("motivo") || "";
    var known = flashMessages[msg];
    if (known) {
      showFlash(known[0], known[1]);
    } else {
      showFlash("alert-danger", motivo || "No se pudo completar la operación.");
    }
    if (window.history && window.history.replaceState) {
      window.history.replaceState(null, "", window.location.pathname);
    }
  })();

  function postAndRedirect(url, body, button) {
    if (button) button.disabled = true;
    fetch(url, { method: "POST", body: body, redirect: "follow" })
      .then(function (response) {
        if (response.redirected && response.url) {
          window.location.replace(response.url);
          return;
        }
        if (button) button.disabled = false;
        showFlash("alert-danger", "El servidor respondió de forma inesperada.");
      })
      .catch(function () {
        if (button) button.disabled = false;
        showFlash("alert-danger", "No se pudo conectar con el servidor.");
      });
  }

  var formModal = document.getElementById("bookFormModal");
  var bookForm = document.getElementById("book-form");
  var formTitle = document.getElementById("bookFormTitle");
  var titleInput = document.getElementById("book-title");
  var authorInput = document.getElementById("book-author");
  var yearInput = document.getElementById("book-year");
  var coverHint = document.getElementById("cover-hint");
  var submitBtn = document.getElementById("book-submit");

  function openForm(mode, book) {
    if (!formModal || !bookForm || !window.bootstrap) return;
    bookForm.reset();
    if (mode === "edit" && book) {
      bookForm.action = "/books/" + encodeURIComponent(book.id) + "/edit";
      formTitle.textContent = "Editar libro";
      titleInput.value = book.title;
      authorInput.value = book.author;
      yearInput.value = book.year;
      coverHint.textContent = "Dejala vacía para conservar la portada actual. Formatos: JPEG, PNG, WebP, GIF o SVG (máx. 5 MB).";
      submitBtn.textContent = "Guardar cambios";
    } else {
      bookForm.action = "/books";
      formTitle.textContent = "Agregar libro";
      coverHint.textContent = "Formatos admitidos: JPEG, PNG, WebP, GIF o SVG (máx. 5 MB).";
      submitBtn.textContent = "Guardar libro";
    }
    submitBtn.disabled = false;
    window.bootstrap.Modal.getOrCreateInstance(formModal).show();
  }

  var addBtn = document.getElementById("add-book-btn");
  if (addBtn) {
    addBtn.addEventListener("click", function () {
      openForm("add");
    });
  }

  Array.prototype.forEach.call(document.querySelectorAll("[data-book-edit]"), function (button) {
    button.addEventListener("click", function () {
      openForm("edit", {
        id: button.dataset.id,
        title: button.dataset.title,
        author: button.dataset.author,
        year: button.dataset.year
      });
    });
  });

  if (bookForm) {
    bookForm.addEventListener("submit", function (event) {
      if (!window.fetch) return;
      event.preventDefault();
      postAndRedirect(bookForm.action, new FormData(bookForm), submitBtn);
    });
  }

  Array.prototype.forEach.call(document.querySelectorAll("[data-book-delete]"), function (form) {
    var button = form.querySelector("button");
    if (!button) return;
    button.addEventListener("click", function () {
      var card = form.closest(".book-card");
      var title = card ? card.getAttribute("data-title") : "";
      if (!window.confirm('¿Eliminar "' + title + '" del catálogo?')) return;
      if (!window.fetch) {
        form.submit();
        return;
      }
      postAndRedirect(form.action, null, button);
    });
  });
})();
