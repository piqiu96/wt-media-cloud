import { createApiClient } from './http.js'
export function createMediaAccountImportClient({base='/api/v1',fetch=globalThis.fetch}={}) {
 const api=createApiClient({base,fetchImpl:fetch}), path=id=>`/media-account-imports/${id}`
 return {preview:input=>api.post('/media-account-imports/preview',input),create:input=>api.post('/media-account-imports',input),list:()=>api.get('/media-account-imports'),get:id=>api.get(path(id)),start:(id,row,input)=>api.post(`${path(id)}/items/${row}/start`,input),result:(id,row,input)=>api.put(`${path(id)}/items/${row}/result`,input),action:(id,row,input)=>api.post(`${path(id)}/items/${row}/actions`,input)}
}
