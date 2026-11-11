(() => {
  if (!("serviceWorker" in navigator)) return;

  window.addEventListener("load", () => {
    navigator.serviceWorker.register("/sw.js?v=unyport-16").catch(() => {});
  });
})();
