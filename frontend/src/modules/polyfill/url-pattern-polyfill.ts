import { URLPattern as URLPatternType } from 'urlpattern-polyfill'

// @ts-ignore: Property 'UrlPattern' does not exist
if (!globalThis.URLPattern) {
    // @ts-ignore: Property 'UrlPattern' does not exist
    globalThis.URLPattern = URLPatternType
}