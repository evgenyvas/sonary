import { legacy_createStore as createStore, combineReducers } from 'redux'
import { ErrorShowMode, type ErrorContext } from './types'
import { notify } from '@/utils/notifier'

const initialError = {
    errorMsg: <string>'',
    errorShowMode: <ErrorShowMode>ErrorShowMode.Notify,
    errorContext: <ErrorContext>{
        url: '',
        status: '',
        statusText: '',
        responseData: '',
    }
}

const SET_ERROR_MESSAGE: string = 'SET_ERROR_MESSAGE'
const SET_ERROR_SHOW_MODE: string = 'SET_ERROR_SHOW_MODE'
const SET_ERROR_CONTEXT: string = 'SET_ERROR_CONTEXT'

export const setErrorMessage = (payload: string) => {
    return {
        type: SET_ERROR_MESSAGE,
        payload
    }
}

export const setErrorShowMode = (payload: ErrorShowMode) => {
    return {
        type: SET_ERROR_SHOW_MODE,
        payload
    }
}

export const setErrorContext = (payload?: ErrorContext) => {
    return {
        type: SET_ERROR_CONTEXT,
        payload
    }
}

export const errorSlice = (state = initialError, action: any) => {
    switch (action.type) {
        case SET_ERROR_MESSAGE:
            state.errorMsg = action.payload
            notify(state.errorMsg)
            break
        case SET_ERROR_SHOW_MODE:
            state.errorShowMode = action.payload
            break
        case SET_ERROR_CONTEXT:
            state.errorContext = action.payload || initialError.errorContext
            break
    }
    return state
}

const store = createStore(combineReducers({
    error: errorSlice,
}))

export type ErrorRootState = ReturnType<typeof store.getState>

export default store
