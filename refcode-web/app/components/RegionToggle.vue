<script setup lang="ts">
// 只對「登入且填了所在地」的人出現 —— 匿名訪客本來就看得到全部，
// 給他們一個切不動的開關只會讓人以為東西壞了。
const { user } = useAuth()
const { showAll, canFilter } = useRegionFilter()
const { countryName } = useCountries()

const regionLabel = computed(() => countryName(user.value?.country ?? ''))
</script>

<template>
  <button
    v-if="canFilter"
    class="text-xs font-semibold text-muted underline-offset-2 hover:text-ink hover:underline"
    @click="showAll = !showAll"
  >
    {{
      showAll
        ? $t('home.regionOnlyMine', { region: regionLabel })
        : $t('home.regionShowAll')
    }}
  </button>
</template>
