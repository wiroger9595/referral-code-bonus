<template>
  <svg viewBox="0 0 48 48" fill="none" aria-hidden="true">
    <!-- 兩枚碼片被雙向箭頭串成一圈＝一組碼在兩個人之間流動。三種語言共用同一個
         記號，所以標誌本身不含站名，站名由呼叫端接在旁邊。
         整個記號吃 currentColor，顏色交給呼叫端的 text-* 決定。

         注意尺寸：幣面那兩道碼線在 24 px 以下會糊成實心圓點，呼叫端不要小於
         32 px。真正的小尺寸（favicon）用的是另一組括號記號，不是這個。 -->
    <mask id="logo-mark-chips">
      <rect width="48" height="48" fill="black" />
      <circle cx="8.5" cy="24" r="6.5" fill="white" />
      <circle cx="39.5" cy="24" r="6.5" fill="white" />
      <g fill="black">
        <rect x="4.5" y="21.1" width="8" height="1.9" rx="0.95" />
        <rect x="4.5" y="25" width="5.4" height="1.9" rx="0.95" />
        <rect x="35.5" y="21.1" width="8" height="1.9" rx="0.95" />
        <rect x="35.5" y="25" width="5.4" height="1.9" rx="0.95" />
      </g>
    </mask>

    <g stroke="currentColor" stroke-width="4" stroke-linecap="round" stroke-linejoin="round">
      <path d="M10.58 16.25A15.5 15.5 0 0 1 33.54 11.79" />
      <path d="M37.42 31.75A15.5 15.5 0 0 1 14.46 36.21" />
    </g>
    <path fill="currentColor" d="M38.27 15.48 36.19 8.40 30.89 15.18Z" />
    <path fill="currentColor" d="M9.73 32.52 11.81 39.60 17.11 32.82Z" />

    <rect width="48" height="48" fill="currentColor" mask="url(#logo-mark-chips)" />
  </svg>
</template>
