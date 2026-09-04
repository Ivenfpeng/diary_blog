(() => {
  const toggle = document.querySelector('[data-menu-toggle]');
  const taxonomy = document.querySelector('[data-taxonomy]');
  if (!toggle || !taxonomy) return;
  toggle.addEventListener('click', () => {
    const expanded = toggle.getAttribute('aria-expanded') === 'true';
    toggle.setAttribute('aria-expanded', String(!expanded));
    taxonomy.classList.toggle('is-open', !expanded);
  });
})();
