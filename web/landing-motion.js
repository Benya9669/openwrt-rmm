(() => {
  const landing = document.querySelector(".landing-view");
  if (!landing) return;

  const reducedMotion = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
  const revealTargets = [
    ...landing.querySelectorAll(
      ".landing-features article, .landing-partners-copy, .partner-logo, .landing-security > *, .landing-login-section > *, .landing-footer > *",
    ),
  ];

  landing.classList.add("motion-ready");
  revealTargets.forEach((element, index) => {
    element.classList.add("landing-reveal");
    element.style.setProperty("--reveal-order", String(index % 3));
  });

  if (reducedMotion || !("IntersectionObserver" in window)) {
    revealTargets.forEach((element) => element.classList.add("is-revealed"));
    return;
  }

  const observer = new IntersectionObserver(
    (entries) => {
      for (const entry of entries) {
        if (!entry.isIntersecting) continue;
        entry.target.classList.add("is-revealed");
        observer.unobserve(entry.target);
      }
    },
    { rootMargin: "0px 0px -8%", threshold: 0.12 },
  );

  revealTargets.forEach((element) => observer.observe(element));
})();
