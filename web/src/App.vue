<template>
  <div class="shell">
    <aside class="side">
      <RouterLink to="/" class="brand">
        <h1>SOMUN</h1>
        <p class="ver">마케팅 코어 · 트랜치 1</p>
      </RouterLink>
      <nav>
        <div v-for="g in menuGroups" :key="g.label" class="group">
          <h4>{{ g.label }}</h4>
          <RouterLink v-for="it in g.items" :key="it.to" :to="it.to">{{ it.label }}</RouterLink>
        </div>
      </nav>
      <p class="who">{{ auth.user ? auth.user.username + ' 로그인 중' : '미로그인 · 설정에서 로그인' }}</p>
    </aside>
    <div class="body">
      <header class="topbar">
        <strong>마케팅 콘솔</strong>
        <span class="muted small">비용 0원 · 로컬 운영</span>
      </header>
      <main class="main">
        <RouterView />
      </main>
    </div>
  </div>
</template>

<script setup>
import { menuGroups } from './router/index.js'
import { auth } from './stores/auth.js'
</script>

<style>
:root {
  --bg: #edf1f5;
  --panel: #ffffff;
  --ink: #17222e;
  --muted: #5d6f80;
  --line: #dbe3ec;
  --soft: #f5f8fb;
  --accent: #0e7490;
  --accent-dark: #0b5d74;
  --accent-soft: #e2f1f6;
  --accent-ink: #ffffff;
  --warn: #b42318;
  --ok: #067647;
  --hold: #8a5a00;
  --hold-soft: #fdf3dd;
}
* { box-sizing: border-box; }
body {
  margin: 0;
  background: var(--bg);
  color: var(--ink);
  font-family: "Malgun Gothic", "Apple SD Gothic Neo", system-ui, sans-serif;
  font-size: 14px;
  line-height: 1.6;
}
.shell { display: flex; min-height: 100vh; }
.side {
  width: 216px;
  flex: none;
  background: var(--panel);
  border-right: 1px solid var(--line);
  padding: 18px 12px 14px;
  display: flex;
  flex-direction: column;
  position: sticky;
  top: 0;
  height: 100vh;
  overflow: auto;
}
.brand { display: block; text-decoration: none; border-radius: 8px; }
.brand h1 { margin: 0; font-size: 21px; letter-spacing: 3px; color: var(--accent-dark); }
.ver { margin: 2px 0 10px; font-size: 12px; color: var(--muted); }
.side nav { flex: 1; }
.group h4 { margin: 16px 8px 5px; font-size: 12px; color: var(--muted); font-weight: 600; }
.side a {
  display: block;
  padding: 7px 10px;
  border-radius: 7px;
  color: var(--ink);
  text-decoration: none;
  font-weight: 500;
}
.side a:hover { background: var(--soft); }
.side a.router-link-active { background: var(--accent-soft); color: var(--accent-dark); font-weight: 700; }
.who { font-size: 12px; color: var(--muted); padding: 0 8px; }
.body { flex: 1; min-width: 0; display: flex; flex-direction: column; }
.topbar {
  background: var(--panel);
  border-bottom: 1px solid var(--line);
  padding: 12px 28px;
  display: flex;
  align-items: baseline;
  gap: 12px;
  position: sticky;
  top: 0;
  z-index: 5;
}
.topbar strong { font-size: 16px; }
.main { flex: 1; padding: 24px 28px 56px; max-width: 1180px; width: 100%; margin: 0 auto; }
.page h2 { margin: 0 0 2px; font-size: 24px; letter-spacing: -0.3px; }
.page > .muted { margin-top: 0; }
.muted { color: var(--muted); }
.small { font-size: 12px; }
.card {
  background: var(--panel);
  border: 1px solid var(--line);
  border-radius: 10px;
  padding: 18px 20px;
  margin: 14px 0;
}
.card.narrow { max-width: 540px; }
.card h3 { margin: 0 0 10px; font-size: 16px; }
.card h4 { margin: 16px 0 8px; font-size: 14px; }
.row { display: flex; gap: 14px; align-items: flex-start; flex-wrap: wrap; }
.grow { flex: 1; min-width: 270px; }
.grow2 { flex: 2; min-width: 330px; }
label { display: block; margin: 10px 0; font-weight: 500; }
input, select, textarea {
  display: block;
  width: 100%;
  margin-top: 5px;
  padding: 8px 10px;
  border: 1px solid var(--line);
  border-radius: 6px;
  font: inherit;
  font-weight: 400;
  background: #fff;
}
input:focus, select:focus, textarea:focus { outline: 2px solid var(--accent-soft); border-color: var(--accent); }
input[type="radio"], input[type="checkbox"] { display: inline; width: auto; margin: 0 6px 0 0; }
textarea { resize: vertical; }
fieldset { border: 1px solid var(--line); border-radius: 6px; margin: 10px 0; }
fieldset label { display: inline-block; margin-right: 18px; font-weight: 400; }
button {
  padding: 8px 16px;
  border: 1px solid var(--accent);
  border-radius: 6px;
  background: var(--accent);
  color: var(--accent-ink);
  font: inherit;
  cursor: pointer;
  margin: 4px 4px 4px 0;
}
button:hover { background: var(--accent-dark); }
button:disabled { opacity: 0.55; cursor: default; }
button.ghost { background: #fff; color: var(--ink); border-color: var(--line); }
button.ghost:hover { background: var(--soft); }
.error { color: var(--warn); }
.ok { color: var(--ok); }
table { width: 100%; border-collapse: collapse; margin-top: 10px; }
th, td { text-align: left; padding: 8px 10px; border-bottom: 1px solid var(--line); vertical-align: top; }
thead th { font-size: 12px; color: var(--muted); background: var(--soft); }
tbody tr:hover { background: var(--soft); }
.ellipsis { max-width: 340px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.toolbar { display: flex; gap: 12px; align-items: flex-end; flex-wrap: wrap; }
.toolbar label { min-width: 170px; }
.picklist { list-style: none; margin: 0; padding: 0; max-height: 440px; overflow: auto; }
.picklist button {
  width: 100%;
  text-align: left;
  background: #fff;
  color: var(--ink);
  border: 1px solid var(--line);
  margin: 0 0 5px;
}
.picklist button:hover { background: var(--soft); }
.picklist button.active { border-color: var(--accent); background: var(--accent-soft); font-weight: 700; }
.badge {
  display: inline-block;
  font-size: 11px;
  padding: 1px 8px;
  border-radius: 999px;
  background: var(--hold-soft);
  color: var(--hold);
  margin-left: 6px;
}
.btnrow { display: flex; gap: 4px; flex-wrap: wrap; align-items: center; }
.formrow { display: flex; gap: 12px; flex-wrap: wrap; align-items: flex-end; }
.formrow label { flex: 1; min-width: 160px; }
.check { display: flex; align-items: center; gap: 4px; font-weight: 400; }
.disclosure, .ailabel { border: 1px dashed var(--line); border-radius: 6px; padding: 10px 12px; margin: 12px 0; background: var(--soft); }
.disclosure strong, .ailabel strong { font-size: 12px; color: var(--muted); }
.disclosure p, .ailabel p { margin: 4px 0 0; }
.thumbs { display: flex; gap: 8px; flex-wrap: wrap; margin: 8px 0; }
.thumbs img { max-height: 120px; border: 1px solid var(--line); border-radius: 6px; }
.thumbs figure { margin: 0; text-align: center; }
.thumbs figcaption { margin-top: 4px; }
.thumbs figcaption button { font-size: 12px; padding: 3px 10px; }
.reading { margin: 8px 0; }
.reading > div { display: flex; gap: 12px; padding: 6px 0; border-bottom: 1px solid var(--line); }
.reading dt { flex: none; width: 110px; color: var(--muted); }
.reading dd { margin: 0; white-space: pre-wrap; }
.reading .unknowns dt, .reading .unknowns dd { color: var(--hold); }
.jobs { list-style: none; margin: 4px 0; padding: 0; font-size: 13px; }
.jobs li { padding: 4px 0; border-bottom: 1px dotted var(--line); }
.modal-back { position: fixed; inset: 0; background: rgba(23, 34, 46, 0.5); display: flex; align-items: center; justify-content: center; }
.modal { background: #fff; border-radius: 12px; padding: 22px 26px; max-width: 480px; width: 92%; }
.modal h3 { margin-top: 0; }
.chatlog { border: 1px solid var(--line); border-radius: 8px; padding: 12px; min-height: 200px; max-height: 340px; overflow: auto; background: var(--soft); }
.chatlog .me { text-align: right; margin: 6px 0; }
.chatlog .them { text-align: left; margin: 6px 0; }
.chatlog p { display: inline-block; padding: 6px 12px; border-radius: 12px; background: #e6ebf1; margin: 0; max-width: 80%; }
.chatlog .me p { background: var(--accent-soft); }
.tabs { display: flex; gap: 6px; margin-bottom: 10px; }
.tabs button { background: #fff; color: var(--ink); border-color: var(--line); }
.tabs button:hover { background: var(--soft); }
.tabs button.active { background: var(--accent); color: #fff; border-color: var(--accent); }
.statgrid { display: grid; grid-template-columns: repeat(auto-fit, minmax(160px, 1fr)); gap: 12px; margin: 12px 0; }
.stat { border: 1px solid var(--line); border-radius: 10px; padding: 14px 16px; background: #fff; }
.stat b { display: block; font-size: 28px; color: var(--accent-dark); }
.stat span { font-size: 12px; color: var(--muted); }
.notice { border: 1px solid var(--line); border-radius: 6px; padding: 10px 14px; margin: 10px 0; background: var(--soft); }
.empty { border: 1px dashed var(--line); border-radius: 8px; padding: 24px; text-align: center; color: var(--muted); background: var(--soft); }
.form-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); gap: 0 14px; }
.form-grid .full { grid-column: 1 / -1; }
.table-wrap { overflow-x: auto; margin: 0 -4px; padding: 0 4px; }
.pill { display: inline-block; font-size: 11px; font-weight: 700; padding: 2px 10px; border-radius: 999px; background: #e6ebf1; color: #3d4c5d; white-space: nowrap; }
.p-draft, .p-pending { background: #e6ebf1; color: #3d4c5d; }
.p-review, .p-claimed { background: #fdf3dd; color: #8a5a00; }
.p-approved { background: #e2f1f6; color: #0b5d74; }
.p-queued { background: #d9f0e3; color: #067647; }
.p-published, .p-done, .p-connected { background: #17222e; color: #fff; }
.p-failed, .p-expired { background: #fbe4e0; color: #b42318; }
.p-limited { background: #fdf3dd; color: #8a5a00; }
.p-held { background: #fbe4e0; color: #b42318; }
.p-inactive_hold { background: #e6ebf1; color: #3d4c5d; }
.p-chief { background: #e2f1f6; color: #0b5d74; }
.p-worker { background: #e6ebf1; color: #3d4c5d; }
.declist { margin: 6px 0; padding-left: 22px; }
.declist li { padding: 5px 0; border-bottom: 1px dotted var(--line); }
.declist li:last-child { border-bottom: none; }
.speclist { list-style: none; margin: 6px 0; padding: 0; }
.speclist li { display: flex; gap: 10px; padding: 7px 0; border-bottom: 1px solid var(--line); }
.speclist li:last-child { border-bottom: none; }
.speclist .k { flex: none; width: 130px; color: var(--muted); }
.sect-head { display: flex; align-items: baseline; justify-content: space-between; gap: 10px; }
.sect-head .count { font-size: 12px; color: var(--muted); }
</style>
