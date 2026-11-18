<template>
  <RouterLink :to="'/promo/products/' + product.id" class="pu-card pu-pcard pu-ccard">
    <img v-if="product.photo" :src="product.photo" :alt="product.name" class="pu-img" />
    <div class="pu-pbody">
      <p class="pu-pname">{{ product.name }} <span v-if="product.status !== 'recruiting'" class="pu-chip neutral">마감</span></p>
      <p class="pu-pmeta">{{ product.store }} · {{ formatWon(product.price) }}</p>
      <p class="pu-pfee pu-fee">{{ formatFee(product.feePerOrder) }}</p>
      <div class="pu-pfoot">
        <span class="pu-dday" :class="{ over: ddayUntil(product.recruitUntil).over }">{{ ddayUntil(product.recruitUntil).text }}</span>
        <span class="pu-applied">신청 {{ product.applicants || 0 }}명</span>
      </div>
    </div>
  </RouterLink>
</template>

<script setup>
import { ddayUntil, formatFee, formatWon } from '../../stores/member.js'

defineProps({ product: { type: Object, required: true } })
</script>
