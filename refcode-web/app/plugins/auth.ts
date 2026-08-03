// SSR 時就把 cookie 裡的 token 換成使用者資料，header 才不會先 render 成未登入、
// hydrate 之後才跳成已登入。沒有 cookie 的訪客不會多打任何 API（見 useAuth 的 restore）。
export default defineNuxtPlugin(async () => {
  await useAuth().restore()
})
