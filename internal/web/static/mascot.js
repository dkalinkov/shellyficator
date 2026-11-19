// The login uses four poses. The header has a fixed head and separate eyelids
// so small-image sampling cannot make the whole mark vibrate during a wink.
(() => {
  const mascots = document.querySelectorAll('[data-mascot]');
  if (!mascots.length || !Element.prototype.animate || !window.IntersectionObserver) return;
  const reducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)');
  const controllers = new Map();

  for (const element of mascots) {
    const full = element.dataset.mascot === 'full';
    const rest = full ? 12000 : 2300;
    const sprite = element.querySelector('.mascot-frames');
    const still = element.querySelector('.mascot-still');
    const sources = full ? [`${basePath}/assets/mascot-full-frames.webp`] : [
      `${basePath}/assets/logo-mascot-base.png`,
      `${basePath}/assets/logo-mascot-eye-open.png`,
      `${basePath}/assets/logo-mascot-eye-closed.png`,
    ];
    let visible = false;
    let loaded = false;
    let loading = false;
    let failed = false;
    let timer;
    let animations = [];
    let openEye;
    let closedEye;
    const allowed = () => visible && !document.hidden && !reducedMotion.matches;

    function stop() {
      clearTimeout(timer);
      animations.forEach(animation => animation.cancel());
      animations = [];
    }

    function schedule(delay = rest) {
      clearTimeout(timer);
      if (allowed() && loaded) timer = setTimeout(play, delay);
    }

    function play() {
      if (!allowed() || !loaded) return;
      if (!full && animations.some(animation => animation.playState === 'running')) return;
      // Let people type without a mascot asking for their attention.
      if (full && document.activeElement?.matches('input')) {
        schedule();
        return;
      }
      stop();
      const duration = full ? 1600 : 1200;
      if (full) {
        const poses = [[0, 0], [0.31, 0], [0.36, 1], [0.4, 2], [0.44, 3],
          [0.57, 3], [0.62, 2], [0.67, 1], [0.72, 0], [1, 0]];
        animations.push(sprite.animate(poses.map(([offset, frame]) => ({
          offset, backgroundPosition: `${frame * 100 / 3}% 0%`, easing: 'steps(1, end)',
        })), { duration }));
        // A curious lean, the wink, then a tiny pleased hop on those little boots.
        animations.push(element.animate([
          { transform: 'rotate(0deg) translateY(0)', offset: 0 },
          { transform: 'rotate(-4deg) translateY(0)', offset: 0.22 },
          { transform: 'rotate(-4deg) translateY(0)', offset: 0.43 },
          { transform: 'rotate(2deg) translateY(-5px)', offset: 0.7 },
          { transform: 'rotate(0deg) translateY(0)', offset: 1 },
        ].map(pose => ({ ...pose, easing: 'cubic-bezier(0.22, 1, 0.36, 1)' })), { duration }));
      } else {
        // The wink is the resting expression; peek open briefly, then settle.
        animations.push(openEye.animate([
          { clipPath: 'inset(50% 0%)', offset: 0 },
          { clipPath: 'inset(0% 0%)', offset: 0.125 },
          { clipPath: 'inset(0% 0%)', offset: 0.875 },
          { clipPath: 'inset(50% 0%)', offset: 1 },
        ].map(pose => ({ ...pose, easing: 'cubic-bezier(0.4, 0, 0.2, 1)' })), { duration }));
        animations.push(closedEye.animate([
          { opacity: 1, offset: 0 },
          { opacity: 0, offset: 0.125 },
          { opacity: 0, offset: 0.875 },
          { opacity: 1, offset: 1 },
        ], { duration }));
      }
      schedule(duration + rest);
    }

    function sync() {
      if (!allowed()) {
        stop();
        element.classList.remove('is-ready');
        return;
      }
      if (failed || !still.complete || !still.naturalWidth) return;
      if (!loaded) {
        if (loading) return;
        loading = true;
        const images = sources.map(source => {
          const image = new Image();
          image.alt = '';
          image.src = source;
          return image;
        });
        Promise.all(images.map(image => image.decode())).then(() => {
          loaded = true;
          loading = false;
          if (full) {
            sprite.style.backgroundImage = `url("${sources[0]}")`;
          } else {
            const [base, open, closed] = images;
            base.className = 'mascot-head-base';
            open.className = 'mascot-eye-open';
            closed.className = 'mascot-eye-closed';
            openEye = open;
            closedEye = closed;
            const eye = document.createElement('span');
            eye.className = 'mascot-eye';
            eye.append(open, closed);
            sprite.replaceChildren(base, eye);
          }
          sync();
        }).catch(() => { failed = true; loading = false; });
        return;
      }
      if (!element.classList.contains('is-ready')) {
        element.classList.add('is-ready');
        schedule(full ? 1200 : rest);
      }
    }

    controllers.set(element, {
      sync,
      setVisible(value) { visible = value; sync(); },
    });
    still.addEventListener('load', sync);
    const trigger = full ? element : element.closest('a') || element;
    trigger.addEventListener('pointerenter', play);
    trigger.addEventListener('focusin', play);
  }

  const observer = new IntersectionObserver(entries => {
    for (const entry of entries) controllers.get(entry.target).setVisible(entry.isIntersecting);
  }, { threshold: 0.1 });
  controllers.forEach((controller, element) => observer.observe(element));
  const sync = () => controllers.forEach(controller => controller.sync());
  reducedMotion.addEventListener('change', sync);
  document.addEventListener('visibilitychange', sync);
  window.addEventListener('pagehide', () => controllers.forEach(controller => controller.setVisible(false)));
  window.addEventListener('pageshow', () => {
    // Re-observe on a back/forward-cache restore, which preserves this script.
    controllers.forEach((controller, element) => { observer.unobserve(element); observer.observe(element); });
  });
})();
