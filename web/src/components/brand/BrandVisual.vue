<template>
  <div class="bd-visual" role="img" aria-label="SOMUN 콘솔 미리보기">
    <div class="bv-win">
      <div class="bv-bar"><span></span><span></span><span></span></div>
      <div class="bv-body">
        <div class="bv-side">
          <span class="bv-logo"></span>
          <span></span><span></span><span class="on"></span><span></span><span></span>
        </div>
        <div class="bv-main">
          <div class="bv-kpis"><span></span><span></span><span></span></div>
          <div class="bv-chart">
            <span class="bv-cap">발행 추이</span>
            <svg viewBox="0 0 200 56" aria-hidden="true">
              <polyline points="4,48 34,42 64,44 94,30 124,32 154,16 184,18" />
              <circle cx="184" cy="18" r="3.5" />
            </svg>
          </div>
          <div class="bv-rows"><span></span><span></span><span></span></div>
        </div>
      </div>
    </div>
    <p class="bv-cap">콘솔 미리보기</p>
  </div>
</template>

<style scoped>
.bd-visual { margin: 0; }
.bv-win { background: var(--panel); border: 1px solid var(--line); border-radius: 16px; box-shadow: 0 24px 60px -24px rgba(0, 0, 0, 0.18); overflow: hidden; }
.bv-bar { display: flex; gap: 6px; padding: 12px 14px; border-bottom: 1px solid var(--line-soft); }
.bv-bar span { width: 10px; height: 10px; border-radius: 50%; background: var(--line); }
.bv-body { display: flex; min-height: 250px; }
.bv-side { width: 64px; flex: none; border-right: 1px solid var(--line-soft); padding: 14px 12px; display: flex; flex-direction: column; gap: 9px; }
.bv-side span { height: 9px; border-radius: 5px; background: var(--soft); }
.bv-side span.on { background: var(--accent-soft); outline: 2px solid var(--accent-soft); }
.bv-logo { width: 22px !important; height: 22px !important; border-radius: 7px !important; background: var(--accent) !important; margin-bottom: 4px; }
.bv-main { flex: 1; padding: 16px; display: flex; flex-direction: column; gap: 14px; min-width: 0; }
.bv-kpis { display: flex; gap: 10px; }
.bv-kpis span { flex: 1; height: 34px; border-radius: 10px; background: var(--soft); }
.bv-chart { border: 1px solid var(--line-soft); border-radius: 12px; padding: 10px 12px 4px; }
.bv-cap { font-size: 11px; color: var(--muted); font-weight: 700; }
.bv-chart svg { width: 100%; height: 56px; display: block; }
.bv-chart polyline { fill: none; stroke: var(--accent); stroke-width: 3; stroke-linecap: round; stroke-linejoin: round; stroke-dasharray: 260; stroke-dashoffset: 260; animation: draw 1.6s ease-out 0.3s forwards; }
.bv-chart circle { fill: var(--accent); opacity: 0; animation: pop 0.3s ease-out 1.8s forwards; }
.bv-rows { display: flex; flex-direction: column; gap: 8px; }
.bv-rows span { height: 10px; border-radius: 5px; background: var(--soft); }
.bv-rows span:nth-child(2) { width: 82%; }
.bv-rows span:nth-child(3) { width: 68%; }
p.bv-cap { text-align: center; margin: 12px 0 0; font-size: 12px; }
@keyframes draw { to { stroke-dashoffset: 0; } }
@keyframes pop { to { opacity: 1; } }
@media (prefers-reduced-motion: reduce) {
  .bv-chart polyline { animation: none; stroke-dashoffset: 0; }
  .bv-chart circle { animation: none; opacity: 1; }
}
</style>
