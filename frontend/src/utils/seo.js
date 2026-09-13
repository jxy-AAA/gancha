const SITE_NAME = '棱语 OptiTalk'
const DEFAULT_TITLE = `${SITE_NAME}｜光学问答与知识社区`
const DEFAULT_DESC =
  '棱语 OptiTalk 是面向光学学习者与工程师的问答和知识社区，从基础概念到工程实践，沉淀可靠答案与真实经验。'

function upsertMeta(attr, key, content) {
  if (!content) return
  let el = document.head.querySelector(`meta[${attr}="${key}"]`)
  if (!el) {
    el = document.createElement('meta')
    el.setAttribute(attr, key)
    document.head.appendChild(el)
  }
  el.setAttribute('content', content)
}

function upsertLink(rel, href) {
  if (!href) return
  let el = document.head.querySelector(`link[rel="${rel}"]`)
  if (!el) {
    el = document.createElement('link')
    el.setAttribute('rel', rel)
    document.head.appendChild(el)
  }
  el.setAttribute('href', href)
}

// setSeo 设置当前页面的 title / description / canonical / OG，
// 与后端爬虫预渲染页保持一致，避免「给爬虫看的和用户看的不一样」。
export function setSeo({ title, description, path, type } = {}) {
  const finalTitle = title ? `${title}｜${SITE_NAME}` : DEFAULT_TITLE
  const finalDesc = description || DEFAULT_DESC
  document.title = finalTitle

  upsertMeta('name', 'description', finalDesc)
  upsertMeta('property', 'og:site_name', SITE_NAME)
  upsertMeta('property', 'og:type', type || 'website')
  upsertMeta('property', 'og:title', finalTitle)
  upsertMeta('property', 'og:description', finalDesc)

  const url = window.location.origin + (path || window.location.pathname)
  upsertLink('canonical', url)
  upsertMeta('property', 'og:url', url)
}

export { SITE_NAME, DEFAULT_TITLE, DEFAULT_DESC }
