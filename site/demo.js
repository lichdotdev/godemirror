// Puts godemirror's WebAssembly demo in #giowindow, in a frame of its own,
// exposes it to the page as window.demo, and plays the scripted edit once it
// scrolls into view.
(function () {
  const host = document.getElementById("giowindow");
  const frame = document.createElement("iframe");
  frame.src = "demo-frame.html";
  frame.title = "godemirror, live";
  frame.style.cssText = "position:absolute;inset:0;width:100%;height:100%;border:0;display:block";
  host.appendChild(frame);
  const queue = [];
  let ready = false;
  const post = (cmd, arg) => frame.contentWindow.postMessage({ godemirror: { cmd, arg } }, "*");
  const call = (cmd, arg) => (ready ? post(cmd, arg) : queue.push([cmd, arg]));
  window.demo = {
    show: (v) => call("Show", v),
    split: (on) => call("Split", on),
    theme: (i) => call("Theme", i),
    wrap: (on) => call("Wrap", on),
    play: () => call("Play"),
    whenReady: (f) => (ready ? f() : queue.push(f)),
  };
  window.addEventListener("message", (e) => {
    if (e.source !== frame.contentWindow || !e.data || e.data.godemirror !== "ready") return;
    ready = true;
    document.documentElement.classList.add("demo-ready");
    queue.splice(0).forEach((q) => (typeof q === "function" ? q() : post(q[0], q[1])));
  });

  // Play the edit once, when at least half the demo is on screen.
  let played = false;
  new IntersectionObserver((entries) => {
    if (!played && entries.some((e) => e.intersectionRatio >= 0.5)) {
      played = true;
      setTimeout(() => window.demo.play(), 600);
    }
  }, { threshold: [0.5] }).observe(host);

  // Reveal anything marked .reveal as it scrolls in.
  const io = new IntersectionObserver((entries) => {
    entries.forEach((e) => e.isIntersecting && (e.target.classList.add("in"), io.unobserve(e.target)));
  }, { threshold: 0.15 });
  document.querySelectorAll(".reveal").forEach((el) => io.observe(el));
})();
