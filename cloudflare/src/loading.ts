/**
 * The page shown while the demo container is booting.
 *
 * Must be completely self-contained. During a cold start nothing under
 * /assets/* is reachable, because the SPA is served from inside the container
 * that has not started yet — so a single external stylesheet, font or image
 * reference would leave the loading page itself broken.
 */

const SPINNER = `<svg class="spinner" viewBox="0 0 50 50" aria-hidden="true">
  <circle cx="25" cy="25" r="20" fill="none" stroke-width="4" />
</svg>`

export function loadingPage(): string {
  return `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<meta name="robots" content="noindex">
<title>Starting Nginx UI demo…</title>
<style>
  :root { color-scheme: light dark; }
  * { box-sizing: border-box; }
  body {
    margin: 0; min-height: 100vh; display: grid; place-items: center;
    font: 15px/1.6 -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto,
          "Helvetica Neue", Arial, "PingFang SC", "Microsoft YaHei", sans-serif;
    background: #fff; color: #1f2328;
  }
  @media (prefers-color-scheme: dark) {
    body { background: #141414; color: #e6e6e6; }
    .hint { color: #8b949e !important; }
    .bar { background: #262626 !important; }
  }
  .card { width: min(420px, calc(100vw - 48px)); text-align: center; padding: 24px; }
  .spinner { width: 44px; height: 44px; animation: rotate 1.6s linear infinite; }
  .spinner circle {
    stroke: #1677ff; stroke-linecap: round;
    animation: dash 1.4s ease-in-out infinite;
  }
  @keyframes rotate { 100% { transform: rotate(360deg); } }
  @keyframes dash {
    0%   { stroke-dasharray: 1, 150; stroke-dashoffset: 0; }
    50%  { stroke-dasharray: 90, 150; stroke-dashoffset: -24; }
    100% { stroke-dasharray: 90, 150; stroke-dashoffset: -124; }
  }
  h1 { font-size: 17px; font-weight: 600; margin: 20px 0 8px; }
  .hint { font-size: 13px; color: #656d76; margin: 0; }
  .bar { margin-top: 22px; height: 3px; border-radius: 3px; background: #f0f0f0; overflow: hidden; }
  .bar span {
    display: block; height: 100%; width: 35%; border-radius: 3px; background: #1677ff;
    animation: slide 1.5s ease-in-out infinite;
  }
  @keyframes slide {
    0%   { transform: translateX(-100%); }
    100% { transform: translateX(340%); }
  }
  .slow { margin-top: 18px; font-size: 12.5px; color: #656d76; display: none; }
</style>
</head>
<body>
  <main class="card">
    ${SPINNER}
    <h1>Waking the demo up</h1>
    <p class="hint">This instance sleeps when nobody is using it. First request takes a few seconds.</p>
    <div class="bar"><span></span></div>
    <p class="slow" id="slow">Still starting. This can take up to a minute after a new deploy.</p>
  </main>
<script>
(function () {
  var started = Date.now();
  var delay = 700;

  setTimeout(function () {
    var el = document.getElementById('slow');
    if (el) el.style.display = 'block';
  }, 12000);

  function poll() {
    fetch('/__demo/status', { cache: 'no-store' })
      .then(function (r) { return r.ok ? r.json() : { ready: false }; })
      .then(function (s) {
        if (s && s.ready) {
          // Reload rather than navigate, so the deep link the visitor arrived
          // on is preserved.
          location.reload();
          return;
        }
        schedule();
      })
      .catch(schedule);
  }

  function schedule() {
    // Back off gently, capped, so a long boot does not hammer the edge.
    delay = Math.min(delay * 1.3, 4000);
    setTimeout(poll, delay);
  }

  setTimeout(poll, delay);
})();
</script>
</body>
</html>`
}
