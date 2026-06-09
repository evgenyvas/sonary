export interface ErrorContext {
    url: string,
    status: string,
    statusText: string,
    responseData: any,
}

export enum ErrorShowMode {
    Notify = 'notify',
    Alert = 'alert',
    None = 'none'
}
