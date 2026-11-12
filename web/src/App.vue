<template>
  <div class="shell">
    <aside class="side">
      <h1>SOMUN</h1>
      <p class="ver">마케팅 코어 · 트랜치 1</p>
      <nav>
        <div v-for="g in menuGroups" :key="g.label" class="group">
          <h4>{{ g.label }}</h4>
          <RouterLink v-for="it in g.items" :key="it.to" :to="it.to">{{ it.label }}</RouterLink>
        </div>
      </nav>
      <p class="who">{{ auth.user ? auth.user.username + ' 로그인 중' : '미로그인 · 설정에서 로그인' }}</p>
    </aside>
    <main class="main">
      <RouterView />
    </main>
  </div>
</template>

<script setup>
import { menuGroups } from './router/index.js'
import { auth } from './stores/auth.js'
</script>

<style>
:root {
  --bg: #f4f4f2;
  --panel: #ffffff;
  --ink: #1e1e1e;
  --muted: #5c5c5c;
  --line: #d8d8d4;
  --accent: #1f5c3d;
  --accent-ink: #ffffff;
  --warn: #9a2b2b;
  --ok: #1f5c3d;
  --hold: #8a5a00;
}
* { box-sizing: border-box; }
body {
  margin: 0;
  background: var(--bg);
  color: var(--ink);
  font-family: "Malgun Gothic", "Apple SD Gothic Neo", system-ui, sans-serif;
  font-size: 14px;
  line-height: 1.55;
}
.shell { display: flex; min-height: 100vh; }
.side {
  width: 208px;
  flex: none;
  background: #242424;
  color: #ececec;
  padding: 16px 12px;
  display: flex;
  flex-direction: column;
}
.side h1 { margin: 0; font-size: 20px; letter-spacing: 2px; }
.ver { margin: 2px 0 12px; font-size: 12px; color: #bdbdbd; }
.side nav { flex: 1; }
.group h4 { margin: 14px 4px 4px; font-size: 12px; color: #9a9a9a; font-weight: 600; }
.side a {
  display: block;
  padding: 5px 8px;
  border-radius: 4px;
  color: #ececec;
  text-decoration: none;
}
.side a.router-link-active { background: #3a3a3a; font-weight: 700; }
.side a:hover { background: #333; }
.who { font-size: 12px; color: #bdbdbd; }
.main { flex: 1; min-width: 0; padding: 20px 24px 48px; }
.page h2 { margin: 0 0 4px; font-size: 22px; }
.muted { color: var(--muted); }
.small { font-size: 12px; }
.card {
  background: var(--panel);
  border: 1px solid var(--line);
  border-radius: 6px;
  padding: 14px 16px;
  margin: 12px 0;
}
.card.narrow { max-width: 520px; }
.card h3 { margin: 0 0 8px; font-size: 16px; }
.card h4 { margin: 14px 0 6px; font-size: 14px; }
.row { display: flex; gap: 12px; align-items: flex-start; flex-wrap: wrap; }
.grow { flex: 1; min-width: 260px; }
.grow2 { flex: 2; min-width: 320px; }
label { display: block; margin: 8px 0; }
input, select, textarea {
  display: block;
  width: 100%;
  margin-top: 4px;
  padding: 7px 9px;
  border: 1px solid var(--line);
  border-radius: 4px;
  font: inherit;
  background: #fff;
}
input[type="radio"], input[type="checkbox"] { display: inline; width: auto; margin: 0 6px 0 0; }
textarea { resize: vertical; }
fieldset { border: 1px solid var(--line); border-radius: 4px; margin: 8px 0; }
fieldset label { display: inline-block; margin-right: 16px; }
button {
  padding: 7px 14px;
  border: 1px solid var(--accent);
  border-radius: 4px;
  background: var(--accent);
  color: var(--accent-ink);
  font: inherit;
  cursor: pointer;
  margin: 4px 4px 4px 0;
}
button:disabled { opacity: 0.55; cursor: default; }
button.ghost { background: #fff; color: var(--ink); border-color: var(--line); }
.error { color: var(--warn); }
.ok { color: var(--ok); }
table { width: 100%; border-collapse: collapse; margin-top: 8px; }
th, td { text-align: left; padding: 6px 8px; border-bottom: 1px solid var(--line); vertical-align: top; }
th { font-size: 12px; color: var(--muted); }
.ellipsis { max-width: 320px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.toolbar { display: flex; gap: 12px; align-items: flex-end; flex-wrap: wrap; }
.toolbar label { min-width: 160px; }
.picklist { list-style: none; margin: 0; padding: 0; max-height: 420px; overflow: auto; }
.picklist button {
  width: 100%;
  text-align: left;
  background: #fff;
  color: var(--ink);
  border: 1px solid var(--line);
  margin: 0 0 4px;
}
.picklist button.active { border-color: var(--accent); font-weight: 700; }
.badge {
  display: inline-block;
  font-size: 11px;
  padding: 1px 7px;
  border-radius: 10px;
  background: #f3e3bb;
  color: var(--hold);
  margin-left: 6px;
}
.btnrow { display: flex; gap: 4px; flex-wrap: wrap; align-items: center; }
.formrow { display: flex; gap: 10px; flex-wrap: wrap; align-items: flex-end; }
.formrow label { flex: 1; min-width: 150px; }
.check { display: flex; align-items: center; gap: 4px; }
.disclosure, .ailabel { border: 1px dashed var(--line); border-radius: 4px; padding: 8px 10px; margin: 10px 0; }
.disclosure strong, .ailabel strong { font-size: 12px; color: var(--muted); }
.disclosure p, .ailabel p { margin: 4px 0 0; }
.thumbs { display: flex; gap: 8px; flex-wrap: wrap; margin: 8px 0; }
.thumbs img { max-height: 120px; border: 1px solid var(--line); border-radius: 4px; }
.reading { margin: 8px 0; }
.reading > div { display: flex; gap: 10px; padding: 5px 0; border-bottom: 1px solid var(--line); }
.reading dt { flex: none; width: 110px; color: var(--muted); }
.reading dd { margin: 0; white-space: pre-wrap; }
.reading .unknowns dt, .reading .unknowns dd { color: var(--hold); }
.jobs { list-style: none; margin: 4px 0; padding: 0; font-size: 13px; }
.jobs li { padding: 3px 0; border-bottom: 1px dotted var(--line); }
.modal-back { position: fixed; inset: 0; background: rgba(0,0,0,0.45); display: flex; align-items: center; justify-content: center; }
.modal { background: #fff; border-radius: 8px; padding: 20px 24px; max-width: 460px; width: 92%; }
.modal h3 { margin-top: 0; }
.chatlog { border: 1px solid var(--line); border-radius: 4px; padding: 10px; min-height: 180px; max-height: 320px; overflow: auto; background: #fafafa; }
.chatlog .me { text-align: right; margin: 6px 0; }
.chatlog .them { text-align: left; margin: 6px 0; }
.chatlog p { display: inline-block; padding: 5px 10px; border-radius: 10px; background: #e9e9e6; margin: 0; max-width: 80%; }
.chatlog .me p { background: #d7e7dc; }
.tabs { display: flex; gap: 6px; margin-bottom: 8px; }
.tabs button { background: #fff; color: var(--ink); border-color: var(--line); }
.tabs button.active { background: var(--accent); color: #fff; border-color: var(--accent); }
.statgrid { display: grid; grid-template-columns: repeat(auto-fit, minmax(150px, 1fr)); gap: 10px; margin: 10px 0; }
.stat { border: 1px solid var(--line); border-radius: 6px; padding: 10px 12px; background: #fff; }
.stat b { display: block; font-size: 24px; }
.stat span { font-size: 12px; color: var(--muted); }
.notice { border: 1px solid var(--line); border-radius: 4px; padding: 8px 12px; margin: 8px 0; background: #fbfaf7; }
</style>
