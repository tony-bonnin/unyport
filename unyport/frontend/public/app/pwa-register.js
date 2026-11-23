(() => {
  if (!("serviceWorker" in navigator)) return;

  window.addEventListener("load", () => {
    navigator.serviceWorker.register("/sw.js?v=unyport-22", { updateViaCache: "none" })
      .then((registration) => registration.update())
      .catch(() => {});
  });
})();
