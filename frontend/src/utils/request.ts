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

export default instance
