import { legacyOperationURL, operationURL } from '../routes.js'
location.replace(legacyOperationURL(new URL(location.href)) || operationURL('plans'))
