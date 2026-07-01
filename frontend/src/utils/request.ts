import { $fetch, type $Fetch } from 'ofetch'
import errorStore, { setErrorMessage, setErrorContext } from '@/modules/error/store'

const errors: { [key: string]: string } = {
  '408': 'Request Timeout',
  '409': 'Conflict',
  '425': 'Too Early',
  '429': 'Too Many Requests',
  '500': 'Internal Server Error',
  '502': 'Bad Gateway',
  '503': 'Service Unavailable',
  '504': 'Gateway Timeout',
}

const instance: $Fetch = $fetch.create({
  baseURL: `${import.meta.env.VITE_API_URL}`,
  headers: { 'X-AUTH-TOKEN': import.meta.env.VITE_AUTH_TOKEN },
  timeout: 10000,
  async onRequestError({ error }) {
    let errorMsg: string = String(error)
    errorStore.dispatch(setErrorMessage(errorMsg))
    errorStore.dispatch(setErrorContext())
    console.error(errorMsg)
    return Promise.reject(errorMsg)
  },
  async onResponseError({ response }) {
    errorStore.dispatch(setErrorContext({
      url: response.url,
      status: String(response.status),
      statusText: response.statusText,
      responseData: response._data,
    }))
    let errorMsg: string = ''
    if (response._data.message) {
      errorMsg = response._data.message
      if (response._data.details) {
        errorMsg += ': ' + response._data.details.join(', ')
      }
    } else if (Object.keys(errors).includes(String(response.status))) {
      errorMsg = 'Internal Server Error - ' + response.status + ': ' + errors[response.status]
    } else {
      errorMsg = response._data
    }
    errorStore.dispatch(setErrorMessage(errorMsg))
    console.error(errorMsg)
    return Promise.reject(errorMsg)
  },
})

export function flatten(
  value: unknown,
  prefix = '',
): Record<string, string | number | boolean | (string | number | boolean)[]> {
  const result: Record<
    string,
    string | number | boolean | (string | number | boolean)[]
  > = {}

  if (value === null || value === undefined) {
    return result
  }

  if (typeof value !== 'object') {
    result[prefix] = value as any
    return result
  }

  for (const [key, val] of Object.entries(value)) {
    const fullKey = prefix ? `${prefix}.${key}` : key

    if (val === null || val === undefined) {
      continue
    }

    if (Array.isArray(val)) {
      // ofetch convert array into:
      // key=a&key=b&key=c
      result[fullKey] = val as any
      continue
    }

    if (typeof val === 'object') {
      Object.assign(result, flatten(val, fullKey))
      continue
    }

    result[fullKey] = val as any
  }

  return result
}

export default instance
