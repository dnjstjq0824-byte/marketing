<template>
  <div class="ostrip" role="listbox" aria-label="오피스 현황">
    <div class="ofloor" aria-hidden="true"></div>
    <button
      v-for="p in people"
      :key="p.id"
      role="option"
      :aria-selected="p.id === selectedId"
      class="operson"
      :class="[{ selected: p.id === selectedId }, 'st-' + p.state]"
      @click="$emit('select', p.id)"
    >
      <span class="odot" :class="'st-' + p.state"></span>
      <span class="oavatar">{{ p.initial }}</span>
      <span class="oplate">
        <strong>{{ p.name }}</strong>
        <small>{{ p.sub }}</small>
      </span>
      <span class="ostate">{{ p.stateLabel }}</span>
    </button>
    <p v-if="!people.length" class="oempty">등록된 계정이 없습니다</p>
  </div>
</template>

<script setup>
// Office strip: one desk per account, live state, click to jump to its card.
// States follow the Hakoniwa/Office pattern: working · blocked · done · idle.
defineProps({
  people: { type: Array, default: () => [] },
  selectedId: { type: [Number, String], default: null },
})
defineEmits(['select'])
</script>

<style scoped>
.ostrip {
  position: relative;
  display: flex;
  gap: 10px;
  overflow-x: auto;
  scroll-snap-type: x proximity;
  padding: 14px 14px 18px;
  background: var(--panel);
  border: 1px solid var(--line);
  border-radius: var(--r-lg);
  box-shadow: var(--shadow);
}
.ofloor {
  position: absolute;
  left: 0;
  right: 0;
  bottom: 0;
  height: 26px;
  background: linear-gradient(var(--line-soft), var(--line));
  border-top: 1px solid var(--line);
  pointer-events: none;
}
.operson {
  position: relative;
  scroll-snap-align: start;
  flex: 0 0 auto;
  display: grid;
  grid-template-columns: auto auto;
  grid-template-areas: 'dot state' 'avatar plate';
  gap: 2px 8px;
  align-items: center;
  min-width: 148px;
  max-width: 190px;
  padding: 8px 10px;
  background: var(--soft);
  border: 1px solid var(--line);
  border-radius: var(--r-md);
  cursor: pointer;
  font: inherit;
  color: inherit;
  text-align: left;
}
.operson.selected {
  border-color: var(--accent);
  box-shadow: 0 0 0 2px var(--accent-soft);
}
.odot {
  grid-area: dot;
  width: 9px;
  height: 9px;
  border-radius: 50%;
  background: #8a94a6;
}
.odot.st-working { background: #2f80ed; }
.odot.st-blocked { background: #f28c28; }
.odot.st-done { background: #27ae60; }
.odot.st-working { animation: opulse 1.6s ease-in-out infinite; }
.oavatar {
  grid-area: avatar;
  width: 34px;
  height: 34px;
  border-radius: 50%;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-weight: 700;
  font-size: 15px;
  color: #fff;
  background: var(--pine-soft);
}
.st-blocked .oavatar { background: #9a5b13; }
.st-working .oavatar { background: var(--accent-dark); }
.st-done .oavatar { background: var(--ok); }
.oplate { grid-area: plate; min-width: 0; }
.oplate strong {
  display: block;
  font-size: 13px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.oplate small {
  display: block;
  color: var(--muted);
  font-size: 11px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.ostate {
  grid-area: state;
  justify-self: end;
  font-size: 11px;
  color: var(--ink-soft);
}
.oempty {
  margin: 0;
  padding: 8px 4px;
  color: var(--muted);
  font-size: 13px;
}
@keyframes opulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.35; }
}
@media (prefers-reduced-motion: reduce) {
  .odot.st-working { animation: none; }
  .ostrip { scroll-snap-type: none; }
}
@media (max-width: 640px) {
  .operson { min-width: 132px; padding: 7px 8px; }
  .oavatar { width: 30px; height: 30px; font-size: 13px; }
}
</style>
