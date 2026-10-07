import './style.css';
import { GetConfig, SaveConfig, DefaultConfig, GetLive, Preview, Quit } from '../wailsjs/go/main/App';
import { EventsOn } from '../wailsjs/runtime/runtime';
import { main } from '../wailsjs/go/models';

type Config = main.Config;
type Live = main.LiveState;

const ART = 'https://cdn.cloudflare.steamstatic.com/steam/apps/1621690/header.jpg';

// 10x10 pixel icons drawn for each situation ('#' = main colour, 'a' = amber).
const PIX: Record<string, string[]> = {
  menu: ['..........', '.########.', '.#......#.', '.#.####.#.', '.#......#.', '.#.###..#.', '.#......#.', '.#.####.#.', '.#......#.', '.########.'],
  exploring: ['.aaaaa....', 'a.....a...', '......#a..', '.....#..a.', '....#.....', '...#......', '..#.......', '.#........', '#.........', '..........'],
  base: ['....aa....', '...a..a...', '....aa....', '...#..#...', '..##..##..', '.########.', '..........', '..######..', '.########.', '..........'],
  dungeon: ['..........', '.########.', '.#.#..#.#.', '.#.#..#.#.', '.#.#..#.#.', '.#.#..#.#.', '.#.#..#.#.', '.#.#..#.#.', '##########', '##########'],
  boss: ['..######..', '.########.', '.#..##..#.', '.#..##..#.', '.########.', '..######..', '..#.##.#..', '..######..', '..........', 'aa......aa'],
  dead: ['...####...', '..#....#..', '.#......#.', '.#.#..#.#.', '.#......#.', '.#..##..#.', '.#......#.', '.#......#.', '##########', '##########'],
  options: ['....##....', '.#.####.#.', '..######..', '.###..###.', '###....###', '###....###', '.###..###.', '..######..', '.#.####.#.', '....##....'],
};
function pix(id: string) {
  let rects = '';
  PIX[id].forEach((row, y) => [...row].forEach((c, x) => {
    if (c !== '.') rects += `<rect x="${x}" y="${y}" width="1" height="1" fill="${c === 'a' ? 'var(--amber)' : 'currentColor'}"/>`;
  }));
  return `<svg class="pix" viewBox="0 0 10 10" aria-hidden="true">${rects}</svg>`;
}

// Every in-world situation offers the same tokens; each list starts with the ones that matter there.
const YOU = ['biome', 'place', 'boss', 'hp', 'maxhp', 'food', 'mana', 'maxmana', 'held', 'distance', 'x', 'y', 'partytext', 'party', 'world', 'character'];
const order = (first: string[]) => [...first, ...YOU.filter(t => !first.includes(t))];

const SITS = [
  { id: 'menu', name: 'Main menu', desc: 'The game is open but you have not entered a world.', tokens: [] as string[] },
  { id: 'exploring', name: 'Exploring', desc: 'Out in a biome, digging, with nothing special going on.',
    tokens: order(['biome', 'held', 'distance', 'hp', 'maxhp']) },
  { id: 'base', name: 'At base', desc: 'At your home base or in another safe zone.',
    tokens: order(['biome', 'food', 'partytext', 'world']) },
  { id: 'dungeon', name: 'Dungeon', desc: 'Inside a special area such as the Ancient City or a Mold Dungeon.',
    tokens: order(['place', 'hp', 'maxhp', 'held']) },
  { id: 'boss', name: 'Boss fight', desc: 'Boss music is playing. {boss} is the boss name exactly as the game writes it.',
    tokens: order(['boss', 'hp', 'maxhp', 'mana', 'maxmana', 'held']) },
  { id: 'dead', name: 'Knocked out', desc: 'Your health has hit zero.',
    tokens: order(['biome', 'distance', 'world']) },
];

let cfg: Config;
let live: Live | null = null;
let page = 'exploring';
let saveTimer: number | undefined;
let toastTimer: number | undefined;
let lastInput: HTMLInputElement | null = null; // where token buttons insert
let previewSeq = 0; // drops preview replies that arrive out of order
let pageSeq = 0;
const app = document.getElementById('app')!;
const esc = (s: string) => s.replace(/[&<>"]/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]!));
const running = () => !!live && live.game !== 'closed';
const isLive = (id: string) => running() && live!.situation === id;
const len = (s: string) => [...s].length;

function navHtml() {
  const dcOn = live?.discord === 'connected';
  return `
  <nav class="nav">
    <div class="brand"><div class="px">Core Keeper<br><span>Presence</span></div><small>What Discord says while you play</small></div>
    <div class="nav-list">
      ${SITS.map(s => `<button class="${page === s.id ? 'sel' : ''}" data-page="${s.id}">${pix(s.id)}<span class="px">${s.name}</span>${isLive(s.id) ? '<span class="now px">NOW</span>' : ''}</button>`).join('')}
    </div>
    <div class="nav-foot">
      <button class="${page === 'options' ? 'sel' : ''}" data-page="options">${pix('options')}<span class="px">Options</span></button>
      <div class="conn">
        <span><i class="${running() ? 'on' : ''}"></i>Core Keeper ${running() ? 'running' : 'closed'}</span>
        <span title="${esc(live?.discord ?? '')}"><i class="${dcOn ? 'on' : ''}"></i>Discord ${dcOn ? 'connected' : 'not connected'}</span>
      </div>
    </div>
  </nav>`;
}

function cardHtml(lines: { details: string; state: string }, example: boolean) {
  if (!cfg.enabled) return `<div class="dcard off">Hidden. Turn on “Show on Discord” in Options.</div>`;
  return `
  <div class="dcard">
    <div class="k">PLAYING A GAME${example ? '<span class="ex">Example</span>' : ''}</div>
    <div class="body">
      ${cfg.showImage ? `<img src="${ART}" alt="">` : ''}
      <div>
        <div class="n">Core Keeper</div>
        <div class="d">${esc(len(lines.details.trim()) >= 2 ? lines.details : 'Playing')}</div>
        ${len(lines.state || '') >= 2 ? `<div class="d">${esc(lines.state)}</div>` : ''}
        ${cfg.showTimer ? '<div class="t">12:34 elapsed</div>' : ''}
      </div>
    </div>
  </div>`;
}

function editorHtml() {
  if (page === 'options') {
    const row = (key: 'enabled' | 'showTimer' | 'showImage', title: string, sub: string) => `
      <div class="opt"><div><b>${title}</b><small>${sub}</small></div>
      <button class="toggle" role="switch" aria-checked="${cfg[key]}" aria-label="${title}" data-opt="${key}"></button></div>`;
    return `
      <h1>Options</h1>
      <p class="sub">These apply to every situation.</p>
      <div class="mini-stage" data-preview></div>
      <div class="section">
        ${row('enabled', 'Show on Discord', 'Switch your Core Keeper status on or off without closing the app.')}
        ${row('showTimer', 'Time elapsed', 'Show how long you have been in the current world.')}
        ${row('showImage', 'Game picture', 'Show the Core Keeper artwork beside your status.')}
      </div>
      <div class="section">
        <h2>Update speed</h2>
        <p class="hint">How quickly Discord catches up with the game. Faster shows health and items changing more closely; Discord takes at most 5 updates in 20 seconds, so after a burst of changes the app waits a moment, then sends your latest status.</p>
        <div class="speeds" role="radiogroup" aria-label="Update speed">
          ${[[3, 'Fast', 'every 3 s'], [15, 'Normal', 'every 15 s'], [30, 'Relaxed', 'every 30 s'], [60, 'Calm', 'every minute']].map(([secs, name, sub]) =>
            `<button role="radio" aria-checked="${(cfg.updateSecs || 3) === secs}" data-speed="${secs}"><span class="px">${name}</span><small>${sub}</small></button>`).join('')}
        </div>
      </div>
      <div class="section">
        <h2>Running in the background</h2>
        <p class="hint">Closing this window keeps your Discord status updating. Open the app again to bring the window back, or quit to stop it and clear your status.</p>
        <div class="actions"><button class="btn" data-quit>Quit app</button></div>
      </div>`;
  }
  const s = SITS.find(x => x.id === page)!;
  const l = cfg.situations[page];
  return `
    <h1>${s.name}</h1>
    <p class="sub">${esc(s.desc)}</p>
    <div class="mini-stage" data-preview></div>
    <div class="section">
      <h2>Wording</h2>
      <div class="field"><label for="f-details">First line</label>
        <input id="f-details" data-field="details" value="${esc(l.details)}" maxlength="128"><span class="count" data-count="details"></span></div>
      <div class="field"><label for="f-state">Second line</label>
        <input id="f-state" data-field="state" value="${esc(l.state)}" maxlength="128" placeholder="Leave empty to hide this line"><span class="count" data-count="state"></span></div>
    </div>
    ${s.tokens.length ? `
    <div class="section">
      <h2>Game info you can insert</h2>
      <div class="tokens">${s.tokens.map(t => `<button data-tok="{${t}}">{${t}}</button>`).join('')}</div>
      <p class="hint">Click one to insert it where your cursor is. It is replaced with live game info. Info that doesn't apply right now, like {boss} outside a fight, is empty, and parts split by “·” disappear when their info is empty.</p>
    </div>` : ''}
    <div class="section"><button class="btn" data-reset>Reset this situation</button></div>`;
}

function stageHtml() {
  return `<aside class="stage"><h2>What friends see</h2><div class="who"></div><div data-preview></div><p class="note"></p><div class="readout"></div></aside>`;
}

function render() {
  app.innerHTML = `<div class="window">${navHtml()}<main class="editor"></main>${stageHtml()}</div><div class="toast" id="toast" role="status"></div>`;
  showPage();
}

// Swap only the editor, with the preview already built, so nothing blinks.
async function showPage() {
  const seq = ++pageSeq;
  previewSeq++; // a preview already on its way is for the old page
  const preview = await previewParts();
  if (seq !== pageSeq) return; // another page was picked meanwhile
  const ed = app.querySelector('.editor')!;
  ed.innerHTML = editorHtml();
  ed.scrollTop = 0;
  paintPreview(preview);
  app.querySelectorAll<HTMLButtonElement>('.nav [data-page]').forEach(b => b.classList.toggle('sel', b.dataset.page === page));
  lastInput = app.querySelector('[data-field]');
  wireNav();
  wireEditor();
}

function wireNav() {
  app.querySelectorAll<HTMLButtonElement>('.nav [data-page]').forEach(b => b.onclick = () => {
    if (page !== b.dataset.page) { page = b.dataset.page!; showPage(); }
  });
}

function wireEditor() {
  app.querySelectorAll<HTMLInputElement>('[data-field]').forEach(inp => {
    inp.onfocus = () => (lastInput = inp);
    inp.oninput = () => { (cfg.situations[page] as any)[inp.dataset.field!] = inp.value; refreshPreview(); queueSave(); };
  });
  app.querySelectorAll<HTMLButtonElement>('[data-tok]').forEach(b => b.onclick = () => {
    const inp = lastInput;
    if (!inp) return;
    const at = inp.selectionStart ?? inp.value.length;
    inp.value = inp.value.slice(0, at) + b.dataset.tok + inp.value.slice(inp.selectionEnd ?? at);
    inp.dispatchEvent(new Event('input'));
    inp.focus();
    inp.setSelectionRange(at + b.dataset.tok!.length, at + b.dataset.tok!.length);
  });
  const reset = app.querySelector<HTMLButtonElement>('[data-reset]');
  if (reset) reset.onclick = async () => {
    cfg.situations[page] = (await DefaultConfig()).situations[page];
    showPage(); queueSave();
  };
  const quit = app.querySelector<HTMLButtonElement>('[data-quit]');
  if (quit) quit.onclick = () => Quit();
  app.querySelectorAll<HTMLButtonElement>('[data-speed]').forEach(b => b.onclick = () => {
    cfg.updateSecs = Number(b.dataset.speed);
    app.querySelectorAll<HTMLButtonElement>('[data-speed]').forEach(x => x.setAttribute('aria-checked', String(x === b)));
    queueSave();
  });
  app.querySelectorAll<HTMLButtonElement>('[data-opt]').forEach(t => t.onclick = () => {
    const key = t.dataset.opt as 'enabled' | 'showTimer' | 'showImage';
    cfg[key] = !cfg[key];
    t.setAttribute('aria-checked', String(cfg[key]));
    refreshPreview(); queueSave();
  });
}

async function previewParts() {
  const sit = page === 'options' ? (running() ? live!.situation : 'exploring') : page;
  const showingLive = isLive(sit);
  const lines = showingLive ? { details: live!.details, state: live!.state } : await Preview(sit, cfg.situations[sit]);
  return { sit, showingLive, lines };
}

function paintPreview(p: { sit: string; showingLive: boolean; lines: { details: string; state: string } }) {
  const name = SITS.find(s => s.id === p.sit)!.name;
  app.querySelectorAll('[data-preview]').forEach(el => (el.innerHTML = cardHtml(p.lines, !p.showingLive)));
  const who = app.querySelector('.stage .who');
  if (who) who.textContent = p.showingLive ? 'Your status right now' : `${name}, with example values`;
  const note = app.querySelector('.stage .note');
  if (note) note.textContent = p.showingLive
    ? 'Live. Updates as you play.'
    : running() ? `You are not in “${name}” right now, so example values are shown.` : 'Start Core Keeper and this switches to your real status.';
  paintReadout();
  (['details', 'state'] as const).forEach(k => {
    const el = app.querySelector(`[data-count="${k}"]`);
    if (el && page !== 'options') el.textContent = `${len(cfg.situations[page][k])}/128`;
  });
}

async function refreshPreview() {
  const seq = ++previewSeq;
  const p = await previewParts();
  if (seq === previewSeq) paintPreview(p);
}

// What each token equals right now, so the template editor is easy to understand.
function paintReadout() {
  const box = app.querySelector('.readout');
  if (!box) return;
  const t = live?.game === 'playing' ? live!.tokens : null;
  box.innerHTML = `<h2>Live game info</h2><div class="who">${t ? 'What each token shows right now' : 'Fills in when you are in a world'}</div>
    <dl>${YOU.map(n => {
      const v = t?.[`{${n}}`] ?? '';
      return `<dt>{${n}}</dt><span class="lead"></span><dd class="${v ? '' : 'none'}">${v ? esc(v) : 'none'}</dd>`;
    }).join('')}</dl>`;
}

function queueSave() {
  clearTimeout(saveTimer);
  saveTimer = window.setTimeout(async () => {
    try { await SaveConfig(cfg); toast('Saved'); }
    catch (e) { toast(`Couldn't save: ${e}`); }
  }, 500);
}

function toast(msg: string) {
  const t = app.querySelector('#toast')!;
  t.textContent = msg; t.classList.add('show');
  clearTimeout(toastTimer);
  toastTimer = window.setTimeout(() => t.classList.remove('show'), 1500);
}

async function start() {
  cfg = await GetConfig();
  live = await GetLive();
  if (live?.game === 'playing') page = live.situation;
  render();
  EventsOn('live', (l: Live) => {
    const moved = live?.situation !== l.situation || live?.game !== l.game || live?.discord !== l.discord;
    live = l;
    if (moved) { app.querySelector('.nav')!.outerHTML = navHtml(); wireNav(); }
    refreshPreview();
  });
}

start();
